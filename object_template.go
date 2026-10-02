// Copyright 2020 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include <stdlib.h>
// #include "object_template.h"
import "C"
import (
	"errors"
	"runtime"
	"unsafe"
)

type PropertyAttribute uint8

const (
	None PropertyAttribute = 0

	ReadOnly PropertyAttribute = 1 << iota

	DontEnum

	DontDelete
)

type ObjectTemplate struct {
	*template
}

func NewObjectTemplate(iso *Isolate) *ObjectTemplate {
	if iso == nil {
		panic("nil Isolate argument not supported")
	}

	tmpl := &template{
		ptr: C.NewObjectTemplate(iso.ptr),
		iso: iso,
	}

	runtime.SetFinalizer(tmpl, (*template).finalizer)
	return &ObjectTemplate{tmpl}
}

func (o *ObjectTemplate) NewInstance(ctx *Context) (*Object, error) {
	if ctx == nil {
		return nil, errors.New("v8go: Context cannot be <nil>")
	}

	rtn := C.ObjectTemplateNewInstance(o.ptr, ctx.ptr)
	runtime.KeepAlive(o)
	return objectResult(ctx, rtn)
}

func (o *ObjectTemplate) SetInternalFieldCount(fieldCount uint32) {
	C.ObjectTemplateSetInternalFieldCount(o.ptr, C.int(fieldCount))
}

func (o *ObjectTemplate) SetAccessorProperty(
	key string,
	get *FunctionTemplate,
	set *FunctionTemplate,
	attributes PropertyAttribute,
) {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))
	var (
		getter C.TemplatePtr
		setter C.TemplatePtr
	)
	if get != nil {
		getter = get.ptr
	}

	if set != nil {
		setter = set.ptr
	}

	C.ObjectTemplateSetAccessorProperty(o.ptr, ckey, getter, setter, C.int(attributes))
}

func (o *ObjectTemplate) InternalFieldCount() uint32 {
	return uint32(C.ObjectTemplateInternalFieldCount(o.ptr))
}

func (o *ObjectTemplate) apply(opts *contextOptions) {
	opts.gTmpl = o
}

func (o *ObjectTemplate) MarkAsUndetectable() {
	C.ObjectTemplateMarkAsUndetectable(o.ptr)
}

func (o *ObjectTemplate) SetCallAsFunctionHandler(callback FunctionCallbackWithError) {
	if callback == nil {
		panic("nil callback argument not supported")
	}

	cbref := o.iso.registerCallback(callback)
	C.ObjectTemplateSetCallAsFunctionHandler(
		o.ptr,
		C.int(cbref),
	)
}
