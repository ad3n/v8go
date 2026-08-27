// Copyright 2021 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include "v8go.h"
import "C"
import (
	"runtime"
	"sync"
)

const pooledFunctionArgs = 8

// functionArgsPool amortizes argument marshalling allocations for the common
// case. Entries are cleared before being returned so the pool never extends
// the lifetime of V8 handles.
var functionArgsPool = sync.Pool{
	New: func() any { return new([pooledFunctionArgs]C.ValuePtr) },
}

type marshalledFunctionArgs struct {
	ptr    *C.ValuePtr
	pooled *[pooledFunctionArgs]C.ValuePtr
	large  []C.ValuePtr
	count  int
}

func marshalFunctionArgs(args []Valuer) marshalledFunctionArgs {
	if len(args) == 0 {
		return marshalledFunctionArgs{}
	}

	if len(args) <= pooledFunctionArgs {
		storage := functionArgsPool.Get().(*[pooledFunctionArgs]C.ValuePtr)
		for i, arg := range args {
			storage[i] = arg.value().ptr
		}
		return marshalledFunctionArgs{
			ptr:    &storage[0],
			pooled: storage,
			count:  len(args),
		}
	}

	storage := make([]C.ValuePtr, len(args))
	for i, arg := range args {
		storage[i] = arg.value().ptr
	}
	return marshalledFunctionArgs{
		ptr:   &storage[0],
		large: storage,
		count: len(args),
	}
}

func (args *marshalledFunctionArgs) release() {
	if args.pooled != nil {
		clear(args.pooled[:args.count])
		functionArgsPool.Put(args.pooled)
		return
	}
	clear(args.large)
	runtime.KeepAlive(args.large)
}

// Function is a JavaScript function.
type Function struct {
	*Value
}

// Call this JavaScript function with the given arguments.
func (fn *Function) Call(recv Valuer, args ...Valuer) (*Value, error) {
	cArgs := marshalFunctionArgs(args)
	defer cArgs.release()
	rtn := C.FunctionCall(fn.ptr, recv.value().ptr, C.int(len(args)), cArgs.ptr)
	return valueResult(fn.ctx, rtn)
}

// Invoke a constructor function to create an object instance.
func (fn *Function) NewInstance(args ...Valuer) (*Object, error) {
	cArgs := marshalFunctionArgs(args)
	defer cArgs.release()
	rtn := C.FunctionNewInstance(fn.ptr, C.int(len(args)), cArgs.ptr)
	return objectResult(fn.ctx, rtn)
}

// Return the source map url for a function.
func (fn *Function) SourceMapUrl() *Value {
	ptr := C.FunctionSourceMapUrl(fn.ptr)
	return &Value{ptr, fn.ctx}
}
