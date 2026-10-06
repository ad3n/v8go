// Copyright 2019 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include "context.h"
import "C"
import (
	"runtime"
	"sync"
	"unsafe"
)

type ctxRef struct {
	ctx      *Context
	refCount int
}

var ctxMutex sync.RWMutex
var ctxRegistry = make(map[int]*ctxRef)
var ctxSeq = 0

type Context struct {
	ref int
	ptr C.ContextPtr
	iso *Isolate
}

type contextOptions struct {
	iso   *Isolate
	gTmpl *ObjectTemplate
}

type ContextOption interface {
	apply(*contextOptions)
}

func NewContext(opt ...ContextOption) *Context {
	opts := contextOptions{}

	for _, o := range opt {
		if o != nil {
			o.apply(&opts)
		}
	}

	if opts.iso == nil {
		opts.iso = NewIsolate()
	}

	if opts.gTmpl == nil {
		opts.gTmpl = &ObjectTemplate{&template{}}
	}

	ctxMutex.Lock()
	ctxSeq++
	ref := ctxSeq
	ctxMutex.Unlock()

	ctx := &Context{
		ref: ref,
		ptr: C.NewContext(opts.iso.ptr, opts.gTmpl.ptr, C.int(ref)),
		iso: opts.iso,
	}

	ctx.register()
	runtime.KeepAlive(opts.gTmpl)
	return ctx
}

func (c *Context) Isolate() *Isolate {
	return c.iso
}

func (c *Context) RetainedValueCount() int {
	ctxMutex.Lock()
	defer ctxMutex.Unlock()
	return int(C.ContextRetainedValueCount(c.ptr))
}

func (c *Context) RunScript(source string, origin string) (*Value, error) {
	cSource := cStringData(source)
	cOrigin := cStringData(origin)
	rtn := C.RunScriptWithLength(c.ptr, cSource, C.int(len(source)), cOrigin, C.int(len(origin)))
	runtime.KeepAlive(source)
	runtime.KeepAlive(origin)
	return valueResult(c, rtn)
}

var emptyStringData byte

func cStringData(s string) *C.char {
	if len(s) == 0 {
		return (*C.char)(unsafe.Pointer(&emptyStringData))
	}

	return (*C.char)(unsafe.Pointer(unsafe.StringData(s)))
}

func (c *Context) Global() *Object {
	valPtr := C.ContextGlobal(c.ptr)
	v := &Value{valPtr, c}

	return &Object{v}
}

func (c *Context) PerformMicrotaskCheckpoint() {
	C.IsolatePerformMicrotaskCheckpoint(c.iso.ptr)
}

func (c *Context) Close() {
	c.deregister()
	C.ContextFree(c.ptr)
	c.ptr = nil
}

func (c *Context) register() {
	ctxMutex.Lock()
	r := ctxRegistry[c.ref]
	if r == nil {
		r = &ctxRef{ctx: c}

		ctxRegistry[c.ref] = r
	}

	r.refCount++
	ctxMutex.Unlock()
}

func (c *Context) deregister() {
	ctxMutex.Lock()
	defer ctxMutex.Unlock()
	r := ctxRegistry[c.ref]
	if r == nil {
		return
	}

	r.refCount--
	if r.refCount <= 0 {
		delete(ctxRegistry, c.ref)
	}
}

func getContext(ref int) *Context {
	ctxMutex.RLock()
	defer ctxMutex.RUnlock()
	r := ctxRegistry[ref]
	if r == nil {
		return nil
	}

	return r.ctx
}

//export goContext
func goContext(ref int) C.ContextPtr {
	ctx := getContext(ref)
	if ctx == nil {
		return nil
	}
	return ctx.ptr
}

func valueResult(ctx *Context, rtn C.RtnValue) (*Value, error) {
	if rtn.value == nil {
		return nil, newJSError(rtn.error)
	}

	return &Value{rtn.value, ctx}, nil
}

func objectResult(ctx *Context, rtn C.RtnValue) (*Object, error) {
	if rtn.value == nil {
		return nil, newJSError(rtn.error)
	}

	return &Object{&Value{rtn.value, ctx}}, nil
}
