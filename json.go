// Copyright 2021 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include <stdlib.h>
// #include "json.h"
import "C"
import (
	"errors"
	"runtime"
	"unsafe"
)

func JSONParse(ctx *Context, str string) (*Value, error) {
	if ctx == nil {
		return nil, errors.New("v8go: Context is required")
	}

	cstr := cStringData(str)
	rtn := C.JSONParseWithLength(ctx.ptr, cstr, C.int(len(str)))
	runtime.KeepAlive(str)
	return valueResult(ctx, rtn)
}

func JSONStringify(ctx *Context, val Valuer) (string, error) {
	if val == nil || val.value() == nil {
		return "", errors.New("v8go: Value is required")
	}

	var ctxPtr C.ContextPtr
	if ctx != nil {
		ctxPtr = ctx.ptr
	}

	str := C.JSONStringify(ctxPtr, val.value().ptr)
	defer C.free(unsafe.Pointer(str))
	return C.GoString(str), nil
}
