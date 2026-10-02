// Copyright 2021 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include <stdlib.h>
// #include "function_template.h"
import "C"
import (
	"runtime"
	"unsafe"
)

type FunctionCallback func(info *FunctionCallbackInfo) *Value

type FunctionCallbackWithError func(info *FunctionCallbackInfo) (*Value, error)

type FunctionCallbackInfo struct {
	ctx  *Context
	args []*Value
	this *Object
}

type ValueError interface {
	error
	Valuer
}

func (i *FunctionCallbackInfo) Context() *Context {
	return i.ctx
}

func (i *FunctionCallbackInfo) This() *Object {
	return i.this
}

func (i *FunctionCallbackInfo) Args() []*Value {
	return i.args
}

func (i *FunctionCallbackInfo) Release() {
	for _, arg := range i.args {
		arg.Release()
	}

	i.this.Release()
}

type FunctionTemplate struct {
	*template
}

func NewFunctionTemplate(iso *Isolate, callback FunctionCallback) *FunctionTemplate {
	if callback == nil {
		panic("nil FunctionCallback argument not supported")
	}

	return NewFunctionTemplateWithError(iso, func(info *FunctionCallbackInfo) (*Value, error) {
		return callback(info), nil
	})
}

func NewFunctionTemplateWithError(
	iso *Isolate,
	callback FunctionCallbackWithError,
) *FunctionTemplate {
	if iso == nil {
		panic("nil Isolate argument not supported")
	}

	if callback == nil {
		panic("nil FunctionCallback argument not supported")
	}

	cbref := iso.registerCallback(callback)

	tmpl := &template{
		ptr: C.NewFunctionTemplate(iso.ptr, C.int(cbref)),
		iso: iso,
	}

	runtime.SetFinalizer(tmpl, (*template).finalizer)
	return &FunctionTemplate{tmpl}
}

func (tmpl *FunctionTemplate) GetFunction(ctx *Context) *Function {
	rtn := C.FunctionTemplateGetFunction(tmpl.ptr, ctx.ptr)
	runtime.KeepAlive(tmpl)
	val, err := valueResult(ctx, rtn)
	if err != nil {
		panic(err)
	}

	return &Function{val}
}

func (tmpl *FunctionTemplate) InstanceTemplate() *ObjectTemplate {
	result := &template{
		ptr: C.FunctionTemplateInstanceTemplate(tmpl.ptr),
		iso: tmpl.iso,
	}

	runtime.SetFinalizer(result, (*template).finalizer)
	return &ObjectTemplate{result}
}

func (tmpl *FunctionTemplate) PrototypeTemplate() *ObjectTemplate {
	result := &template{
		ptr: C.FunctionTemplatePrototypeTemplate(tmpl.ptr),
		iso: tmpl.iso,
	}

	runtime.SetFinalizer(result, (*template).finalizer)
	return &ObjectTemplate{result}
}

func (tmpl *FunctionTemplate) Inherit(base *FunctionTemplate) {
	C.FunctionTemplateInherit(tmpl.ptr, base.ptr)
}

//export goFunctionCallback
func goFunctionCallback(
	ctxref int,
	cbref int,
	thisAndArgs *C.ValuePtr,
	argsCount int,
) (rval C.ValuePtr, rerr C.ValuePtr) {
	ctx := getContext(ctxref)

	this := *thisAndArgs
	info := &FunctionCallbackInfo{
		ctx:  ctx,
		this: &Object{&Value{ptr: this, ctx: ctx}},
		args: make([]*Value, argsCount),
	}

	argv := (*[1 << 30]C.ValuePtr)(unsafe.Pointer(thisAndArgs))[1 : argsCount+1 : argsCount+1]
	for i, v := range argv {
		val := &Value{ptr: v, ctx: ctx}

		info.args[i] = val
	}

	callbackFunc := ctx.iso.getCallback(cbref)
	val, err := callbackFunc(info)
	if err != nil {
		if verr, ok := err.(ValueError); ok {
			return nil, verr.value().ptr
		}

		errv, err := NewValue(ctx.iso, err.Error())
		if err != nil {
			panic(err)
		}

		return nil, errv.ptr
	}

	if val == nil {
		return nil, nil
	}

	return val.ptr, nil
}
