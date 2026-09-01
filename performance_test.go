// Copyright 2026 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"strings"
	"testing"

	v8 "github.com/ad3n/v8go"
)

// These benchmarks model request-time operations separately so production
// users can identify whether isolate, context, compilation, or data transfer is
// the dominant cost in their workload.
func BenchmarkProductionPaths(b *testing.B) {
	b.Run("RunScript/trivial", func(b *testing.B) {
		ctx := v8.NewContext()
		defer ctx.Isolate().Dispose()
		defer ctx.Close()
		b.ReportAllocs()
		for b.Loop() {
			value, err := ctx.RunScript("1 + 2", "request.js")
			if err != nil {
				b.Fatal(err)
			}
			value.Release()
		}
	})

	b.Run("UnboundScript/trivial", func(b *testing.B) {
		iso := v8.NewIsolate()
		defer iso.Dispose()
		ctx := v8.NewContext(iso)
		defer ctx.Close()
		script, err := iso.CompileUnboundScript("1 + 2", "request.js", v8.CompileOptions{})
		if err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			value, err := script.Run(ctx)
			if err != nil {
				b.Fatal(err)
			}
			value.Release()
		}
	})

	for _, size := range []int{128, 4096, 65536} {
		payload := `{"data":"` + strings.Repeat("x", size-11) + `"}`
		b.Run("JSONParse/"+benchmarkSize(size), func(b *testing.B) {
			ctx := v8.NewContext()
			defer ctx.Isolate().Dispose()
			defer ctx.Close()
			b.SetBytes(int64(len(payload)))
			b.ReportAllocs()
			for b.Loop() {
				value, err := v8.JSONParse(ctx, payload)
				if err != nil {
					b.Fatal(err)
				}
				value.Release()
			}
		})
	}
}

func benchmarkSize(size int) string {
	switch size {
	case 128:
		return "128B"
	case 4096:
		return "4KiB"
	case 65536:
		return "64KiB"
	default:
		panic("unhandled benchmark size")
	}
}
