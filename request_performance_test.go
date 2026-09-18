// Copyright 2026 the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"fmt"
	"strings"
	"testing"

	v8 "github.com/ad3n/v8go"
)

// Each iteration owns a fresh isolate and context, including their cleanup.
// The workload uses RunScript, string conversion, Function.Call and JSON APIs.
// It models the request lifecycle, not an application's production trace.
func BenchmarkFreshRequest(b *testing.B) {
	for _, calls := range []int{1, 16} {
		b.Run(fmt.Sprintf("calls_%d", calls), func(b *testing.B) {
			payload := `{"id":"request","data":"` + strings.Repeat("x", 128) + `"}`
			b.ReportAllocs()
			for b.Loop() {
				runFreshRequest(b, payload, "request", calls)
			}
		})
	}
}

func runFreshRequest(tb testing.TB, payload, id string, calls int) {
	tb.Helper()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()
	function, err := ctx.RunScript(`(function(payload, id) {
		if (payload.id !== id) throw new Error("request mismatch");
		globalThis.requestId = id;
		return {id: globalThis.requestId, data: payload.data};
	})`, "request.js")
	if err != nil {
		tb.Fatal(err)
	}
	defer function.Release()
	fn, err := function.AsFunction()
	if err != nil {
		tb.Fatal(err)
	}
	for i := 0; i < calls; i++ {
		input, err := v8.JSONParse(ctx, payload)
		if err != nil {
			tb.Fatal(err)
		}
		requestID, err := v8.NewValue(iso, id)
		if err != nil {
			tb.Fatal(err)
		}
		result, err := fn.Call(v8.Undefined(iso), input, requestID)
		input.Release()
		requestID.Release()
		if err != nil {
			tb.Fatal(err)
		}
		output, err := v8.JSONStringify(ctx, result)
		result.Release()
		if err != nil {
			tb.Fatal(err)
		}
		if output != payload {
			tb.Fatalf("request %q: got %q, want %q", id, output, payload)
		}
	}
	if retained := ctx.RetainedValueCount(); retained != 1 {
		tb.Fatalf("retained %d values; want only the function", retained)
	}
}

// These isolate the changed operations within a request. They do not model
// sharing an isolate or context between requests.
func BenchmarkRequestBoundary(b *testing.B) {
	for _, count := range []int{0, 2, 8, 9, 32} {
		b.Run(fmt.Sprintf("Call/args_%d", count), func(b *testing.B) {
			ctx := v8.NewContext()
			defer ctx.Isolate().Dispose()
			defer ctx.Close()
			value, err := ctx.RunScript(`(function() { return arguments.length; })`, "call.js")
			if err != nil {
				b.Fatal(err)
			}
			defer value.Release()
			fn, err := value.AsFunction()
			if err != nil {
				b.Fatal(err)
			}
			args := make([]v8.Valuer, count)
			for i := range args {
				args[i] = v8.Undefined(ctx.Isolate())
			}
			b.ReportAllocs()
			for b.Loop() {
				result, err := fn.Call(v8.Undefined(ctx.Isolate()), args...)
				if err != nil {
					b.Fatal(err)
				}
				result.Release()
			}
		})
	}
	for _, count := range []int{8, 9} {
		b.Run(fmt.Sprintf("NewInstance/args_%d", count), func(b *testing.B) {
			ctx := v8.NewContext()
			defer ctx.Isolate().Dispose()
			defer ctx.Close()
			value, err := ctx.RunScript(`(function() { this.count = arguments.length; })`, "constructor.js")
			if err != nil {
				b.Fatal(err)
			}
			defer value.Release()
			fn, err := value.AsFunction()
			if err != nil {
				b.Fatal(err)
			}
			args := make([]v8.Valuer, count)
			for i := range args {
				args[i] = v8.Undefined(ctx.Isolate())
			}
			b.ReportAllocs()
			for b.Loop() {
				result, err := fn.NewInstance(args...)
				if err != nil {
					b.Fatal(err)
				}
				result.Release()
			}
		})
	}
	for _, size := range []int{0, 128, 4096, 65536} {
		b.Run(fmt.Sprintf("NewString/bytes_%d", size), func(b *testing.B) {
			iso := v8.NewIsolate()
			defer iso.Dispose()
			// Box outside the loop to distinguish conversion from caller boxing.
			var input any = strings.Repeat("x", size)
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				value, err := v8.NewValue(iso, input)
				if err != nil {
					b.Fatal(err)
				}
				value.Release()
			}
		})
	}
}
