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

const pooledFunctionArgs = 32

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

type Function struct {
	*Value
}

func (fn *Function) Call(recv Valuer, args ...Valuer) (*Value, error) {
	cArgs := marshalFunctionArgs(args)
	defer cArgs.release()

	rtn := C.FunctionCall(fn.ptr, recv.value().ptr, C.int(len(args)), cArgs.ptr)
	runtime.KeepAlive(args)
	return valueResult(fn.ctx, rtn)
}

func (fn *Function) NewInstance(args ...Valuer) (*Object, error) {
	cArgs := marshalFunctionArgs(args)
	defer cArgs.release()

	rtn := C.FunctionNewInstance(fn.ptr, C.int(len(args)), cArgs.ptr)
	runtime.KeepAlive(args)
	return objectResult(fn.ctx, rtn)
}

func (fn *Function) SourceMapUrl() *Value {
	ptr := C.FunctionSourceMapUrl(fn.ptr)
	return &Value{ptr, fn.ctx}
}
