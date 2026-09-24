// Copyright 2026 the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	v8 "github.com/ad3n/v8go"
)

func TestFreshRequestIsolation(t *testing.T) {
	for worker := 0; worker < 16; worker++ {
		t.Run(fmt.Sprintf("worker_%d", worker), func(t *testing.T) {
			t.Parallel()
			for request := 0; request < 8; request++ {
				id := fmt.Sprintf("worker-%d-request-%d", worker, request)
				payload := fmt.Sprintf(`{"id":%q,"data":%q}`, id, strings.Repeat(id, 8))
				runFreshRequest(t, payload, id, 4)
			}
		})
	}
}

func TestRequestStringOwnership(t *testing.T) {
	for _, input := range []string{"", "\x00", "before\x00after", "日本語🙂", strings.Repeat("a", 65536)} {
		t.Run(fmt.Sprintf("bytes_%d", len(input)), func(t *testing.T) {
			t.Parallel()
			iso := v8.NewIsolate()
			defer iso.Dispose()
			for i := 0; i < 8; i++ {
				// A copy that is not a static string. Native storage must outlive it.
				source := strings.Clone(input)
				value, err := v8.NewValue(iso, source)
				if err != nil {
					t.Fatal(err)
				}
				source = ""
				runtime.GC()
				got := value.String()
				value.Release()
				if got != input {
					t.Fatalf("string changed after source lifetime: length %d, want %d", len(got), len(input))
				}
			}
		})
	}
	if value, err := v8.NewValue(nil, "text"); value != nil || err == nil {
		t.Fatal("nil isolate must return an error")
	}
}

func TestRequestInvalidUTF8(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()
	value, err := v8.NewValue(iso, "a\xffb")
	if err != nil {
		t.Fatal(err)
	}
	defer value.Release()
	if got := value.String(); got != "a\ufffdb" {
		t.Fatalf("invalid UTF-8 conversion changed: %q", got)
	}
}

func TestFreshRequestAfterTermination(t *testing.T) {
	func() {
		iso := v8.NewIsolate()
		defer iso.Dispose()
		global := v8.NewObjectTemplate(iso)
		stop := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
			info.Release()
			iso.TerminateExecution()
			return nil
		})
		if err := global.Set("stop", stop); err != nil {
			t.Fatal(err)
		}
		ctx := v8.NewContext(iso, global)
		defer ctx.Close()
		// A loop guarantees V8 reaches an interrupt check after the callback.
		value, err := ctx.RunScript(`(function() { stop(); for (;;) {} })`, "terminate.js")
		if err != nil {
			t.Fatal(err)
		}
		defer value.Release()
		fn, err := value.AsFunction()
		if err != nil {
			t.Fatal(err)
		}
		arg := v8.Undefined(iso)
		result, err := fn.Call(arg, arg, arg, arg, arg, arg, arg, arg, arg)
		if result != nil || err == nil || !strings.HasPrefix(err.Error(), "ExecutionTerminated") {
			t.Fatalf("expected termination error, got %v", err)
		}
	}()
	// A terminated request must not poison a subsequent fresh isolate.
	runFreshRequest(t, `{"id":"after-termination","data":"ok"}`, "after-termination", 2)
}

func TestRequestArgumentBoundaries(t *testing.T) {
	for _, count := range []int{0, 1, 2, 7, 8, 9, 31, 32, 33, 256} {
		t.Run(fmt.Sprintf("args_%d", count), func(t *testing.T) {
			t.Parallel()
			ctx := v8.NewContext()
			iso := ctx.Isolate()
			defer iso.Dispose()
			defer ctx.Close()
			value, err := ctx.RunScript(`(function() {
				let total = 0;
				for (let i = 0; i < arguments.length; i++) total += arguments[i];
				if (new.target) this.total = total;
				return total;
			})`, "arguments.js")
			if err != nil {
				t.Fatal(err)
			}
			defer value.Release()
			fn, err := value.AsFunction()
			if err != nil {
				t.Fatal(err)
			}
			throwValue, err := ctx.RunScript(`(function() { throw new Error("expected"); })`, "throw.js")
			if err != nil {
				t.Fatal(err)
			}
			defer throwValue.Release()
			throwFn, err := throwValue.AsFunction()
			if err != nil {
				t.Fatal(err)
			}
			var args []v8.Valuer // nil variadic arguments must work for count == 0.
			for i := 0; i < count; i++ {
				arg, err := v8.NewValue(iso, int32(i+1))
				if err != nil {
					t.Fatal(err)
				}
				defer arg.Release()
				args = append(args, arg)
			}
			want := int32(count * (count + 1) / 2)
			retained := ctx.RetainedValueCount()
			for i := 0; i < 16; i++ {
				result, err := fn.Call(v8.Undefined(iso), args...)
				if err != nil {
					t.Fatal(err)
				}
				got := result.Int32()
				result.Release()
				if got != want {
					t.Fatalf("Call: got %d, want %d", got, want)
				}
				object, err := fn.NewInstance(args...)
				if err != nil {
					t.Fatal(err)
				}
				total, err := object.Get("total")
				if err != nil {
					t.Fatal(err)
				}
				got = total.Int32()
				total.Release()
				object.Release()
				if got != want {
					t.Fatalf("NewInstance: got %d, want %d", got, want)
				}
				if result, err := throwFn.Call(v8.Undefined(iso), args...); result != nil || err == nil {
					t.Fatal("Call must propagate the JavaScript error")
				}
				if result, err := throwFn.NewInstance(args...); result != nil || err == nil {
					t.Fatal("NewInstance must propagate the JavaScript error")
				}
				if got := ctx.RetainedValueCount(); got != retained {
					t.Fatalf("retained value count grew from %d to %d", retained, got)
				}
			}
		})
	}
}

func TestRequestReentrantCallbackIsolation(t *testing.T) {
	for worker := 0; worker < 8; worker++ {
		t.Run(fmt.Sprintf("worker_%d", worker), func(t *testing.T) {
			t.Parallel()
			iso := v8.NewIsolate()
			defer iso.Dispose()
			id := fmt.Sprintf("callback-%d", worker)
			global := v8.NewObjectTemplate(iso)
			var ctx *v8.Context
			var inner *v8.Function
			callback := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
				defer info.Release()
				if info.Context() != ctx || len(info.Args()) != 8 {
					t.Error("callback received a different request context or argument count")
					return nil
				}
				for _, arg := range info.Args() {
					if arg.String() != id {
						t.Error("callback received another request's argument")
					}
				}
				// Re-enter V8 while the outer Call's argument buffer is still live.
				result, err := inner.Call(v8.Undefined(iso), info.Args()[0])
				if err != nil {
					t.Error(err)
					return nil
				}
				if result.String() != id {
					t.Error("nested call returned another request's result")
				}
				result.Release()
				return nil
			})
			if err := global.Set("checkRequest", callback); err != nil {
				t.Fatal(err)
			}
			ctx = v8.NewContext(iso, global)
			defer ctx.Close()
			innerValue, err := ctx.RunScript(`(id => id)`, "inner.js")
			if err != nil {
				t.Fatal(err)
			}
			defer innerValue.Release()
			inner, err = innerValue.AsFunction()
			if err != nil {
				t.Fatal(err)
			}
			outerValue, err := ctx.RunScript(`(function(...args) { checkRequest(...args); return args.join("|"); })`, "outer.js")
			if err != nil {
				t.Fatal(err)
			}
			defer outerValue.Release()
			outer, err := outerValue.AsFunction()
			if err != nil {
				t.Fatal(err)
			}
			arg, err := v8.NewValue(iso, id)
			if err != nil {
				t.Fatal(err)
			}
			defer arg.Release()
			args := []v8.Valuer{arg, arg, arg, arg, arg, arg, arg, arg}
			want := strings.TrimSuffix(strings.Repeat(id+"|", 8), "|")
			retained := ctx.RetainedValueCount()
			for i := 0; i < 32; i++ {
				result, err := outer.Call(v8.Undefined(iso), args...)
				if err != nil {
					t.Fatal(err)
				}
				got := result.String()
				result.Release()
				if got != want {
					t.Fatalf("outer arguments changed across nested call: %q", got)
				}
				if got := ctx.RetainedValueCount(); got != retained {
					t.Fatalf("callback retained values: got %d, want %d", got, retained)
				}
			}
		})
	}
}
