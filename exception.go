// Copyright 2021 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

import (
	// #include <stdlib.h>
	// #include "v8go.h"
	"C"

	"fmt"
	"unsafe"
)

func NewRangeError(iso *Isolate, msg string) *Exception {
	return newExceptionError(iso, C.ERROR_RANGE, msg)
}

func NewReferenceError(iso *Isolate, msg string) *Exception {
	return newExceptionError(iso, C.ERROR_REFERENCE, msg)
}

func NewSyntaxError(iso *Isolate, msg string) *Exception {
	return newExceptionError(iso, C.ERROR_SYNTAX, msg)
}

func NewTypeError(iso *Isolate, msg string) *Exception {
	return newExceptionError(iso, C.ERROR_TYPE, msg)
}

func NewWasmCompileError(iso *Isolate, msg string) *Exception {
	return newExceptionError(iso, C.ERROR_WASM_COMPILE, msg)
}

func NewWasmLinkError(iso *Isolate, msg string) *Exception {
	return newExceptionError(iso, C.ERROR_WASM_LINK, msg)
}

func NewWasmRuntimeError(iso *Isolate, msg string) *Exception {
	return newExceptionError(iso, C.ERROR_WASM_RUNTIME, msg)
}

func NewError(iso *Isolate, msg string) *Exception {
	return newExceptionError(iso, C.ERROR_GENERIC, msg)
}

func newExceptionError(iso *Isolate, typ C.ErrorTypeIndex, msg string) *Exception {
	cmsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cmsg))
	eptr := C.NewValueError(iso.ptr, typ, cmsg)
	if eptr == nil {
		panic(fmt.Errorf("invalid error type index: %d", typ))
	}

	return &Exception{&Value{ptr: eptr}}
}

type Exception struct {
	*Value
}

func (e *Exception) value() *Value {
	return e.Value
}

func (e *Exception) Error() string {
	return e.String()
}

func (e *Exception) As(target any) bool {
	ep, ok := target.(**Exception)
	if !ok {
		return false
	}

	*ep = e
	return true
}

func (e *Exception) Is(err error) bool {
	eerr, ok := err.(*Exception)
	if !ok {
		return false
	}

	return eerr.String() == e.String()
}

func (e *Exception) String() string {
	if e.Value == nil {
		return "<nil>"
	}

	s := C.ExceptionGetMessageString(e.ptr)
	defer C.free(unsafe.Pointer(s))
	return C.GoString(s)
}
