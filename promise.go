// Copyright 2021 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include <stdlib.h>
// #include "v8go.h"
import "C"
import (
	"errors"
)

type PromiseState int

const (
	Pending PromiseState = iota
	Fulfilled
	Rejected
)

type PromiseResolver struct {
	*Object
	prom *Promise
}

type Promise struct {
	*Object
}

func NewPromiseResolver(ctx *Context) (*PromiseResolver, error) {
	if ctx == nil {
		return nil, errors.New("v8go: Context is required")
	}

	rtn := C.NewPromiseResolver(ctx.ptr)
	obj, err := objectResult(ctx, rtn)
	if err != nil {
		return nil, err
	}

	return &PromiseResolver{obj, nil}, nil
}

func (r *PromiseResolver) GetPromise() *Promise {
	if r.prom == nil {
		ptr := C.PromiseResolverGetPromise(r.ptr)
		val := &Value{ptr, r.ctx}

		r.prom = &Promise{&Object{val}}
	}

	return r.prom
}

func (r *PromiseResolver) Resolve(val Valuer) bool {
	return C.PromiseResolverResolve(r.ptr, val.value().ptr) != 0
}

func (r *PromiseResolver) Reject(err *Value) bool {
	return C.PromiseResolverReject(r.ptr, err.ptr) != 0
}

func (p *Promise) State() PromiseState {
	return PromiseState(C.PromiseState(p.ptr))
}

func (p *Promise) Result() *Value {
	ptr := C.PromiseResult(p.ptr)
	val := &Value{ptr, p.ctx}

	return val
}

func (p *Promise) Then(cbs ...FunctionCallback) *Promise {
	cbwes := make([]FunctionCallbackWithError, len(cbs))
	for i, cb := range cbs {
		cb := cb
		cbwes[i] = func(info *FunctionCallbackInfo) (*Value, error) {
			return cb(info), nil
		}
	}

	return p.ThenWithError(cbwes...)
}

func (p *Promise) ThenWithError(cbs ...FunctionCallbackWithError) *Promise {
	var rtn C.RtnValue
	switch len(cbs) {
	case 1:
		cbID := p.ctx.iso.registerCallback(cbs[0])
		rtn = C.PromiseThen(p.ptr, C.int(cbID))
	case 2:
		cbID1 := p.ctx.iso.registerCallback(cbs[0])
		cbID2 := p.ctx.iso.registerCallback(cbs[1])
		rtn = C.PromiseThen2(p.ptr, C.int(cbID1), C.int(cbID2))

	default:
		panic("1 or 2 callbacks required")
	}

	obj, err := objectResult(p.ctx, rtn)
	if err != nil {
		panic(err)
	}

	return &Promise{obj}
}

func (p *Promise) Catch(cb FunctionCallback) *Promise {
	return p.CatchWithError(func(info *FunctionCallbackInfo) (*Value, error) {
		return cb(info), nil
	})
}

func (p *Promise) CatchWithError(cb FunctionCallbackWithError) *Promise {
	cbID := p.ctx.iso.registerCallback(cb)
	rtn := C.PromiseCatch(p.ptr, C.int(cbID))
	obj, err := objectResult(p.ctx, rtn)
	if err != nil {
		panic(err)
	}

	return &Promise{obj}
}
