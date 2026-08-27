// Copyright 2026 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

import (
	"fmt"
	"testing"
	"unsafe"
)

var benchmarkFunctionArgsSink unsafe.Pointer

func benchmarkFunctionArgs(count int) []Valuer {
	args := make([]Valuer, count)
	for i := range args {
		args[i] = &Value{}
	}
	return args
}

func TestMarshalFunctionArgs(t *testing.T) {
	args := []Valuer{&Value{}, &Value{}}

	marshalled := marshalFunctionArgs(args)
	if marshalled.ptr == nil || marshalled.pooled == nil {
		t.Fatal("small argument list did not use pooled storage")
	}
	marshalled.release()
}

func TestMarshalFunctionArgsSteadyStateAllocations(t *testing.T) {
	for _, count := range []int{0, 1, 2, pooledFunctionArgs} {
		args := benchmarkFunctionArgs(count)
		t.Run(fmt.Sprintf("args_%d", count), func(t *testing.T) {
			warmup := marshalFunctionArgs(args)
			warmup.release()

			allocations := testing.AllocsPerRun(100, func() {
				marshalled := marshalFunctionArgs(args)
				marshalled.release()
			})
			if allocations != 0 {
				t.Fatalf("argument marshalling allocated %v times per run; want zero", allocations)
			}
		})
	}
}

func BenchmarkMarshalFunctionArgs(b *testing.B) {
	for _, count := range []int{0, 1, 2, pooledFunctionArgs, pooledFunctionArgs + 1, 32} {
		args := benchmarkFunctionArgs(count)
		b.Run(fmt.Sprintf("args_%d", count), func(b *testing.B) {
			// Populate sync.Pool before allocation accounting begins. Pools are an
			// amortized optimization and may be emptied by any garbage collection.
			warmup := marshalFunctionArgs(args)
			warmup.release()

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				marshalled := marshalFunctionArgs(args)
				marshalled.release()
			}
		})
	}
}

// BenchmarkMarshalFunctionArgsWithoutPool models the allocation strategy used
// before functionArgsPool. It is retained as a benchmark-only performance
// baseline so regressions can be measured against the old behavior.
func BenchmarkMarshalFunctionArgsWithoutPool(b *testing.B) {
	for _, count := range []int{1, 2, pooledFunctionArgs} {
		args := benchmarkFunctionArgs(count)
		b.Run(fmt.Sprintf("args_%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				storage := make([]unsafe.Pointer, len(args))
				for i, arg := range args {
					storage[i] = unsafe.Pointer(arg.value().ptr)
				}
				benchmarkFunctionArgsSink = unsafe.Pointer(&storage[0])
			}
		})
	}
}
