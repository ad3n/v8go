// Copyright 2021 the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include <stdlib.h>
// #include "unbound_script.h"
import "C"
import "unsafe"

type UnboundScript struct {
	ptr C.UnboundScriptPtr
	iso *Isolate
}

func (u *UnboundScript) Run(ctx *Context) (*Value, error) {
	if ctx.Isolate() != u.iso {
		panic("attempted to run unbound script in a context that belongs to a different isolate")
	}

	rtn := C.UnboundScriptRun(ctx.ptr, u.ptr)
	return valueResult(ctx, rtn)
}

func (u *UnboundScript) CreateCodeCache() *CompilerCachedData {
	rtn := C.UnboundScriptCreateCodeCache(u.iso.ptr, u.ptr)

	cachedData := &CompilerCachedData{
		Bytes:    []byte(C.GoBytes(unsafe.Pointer(rtn.data), rtn.length)),
		Rejected: int(rtn.rejected) == 1,
	}

	C.ScriptCompilerCachedDataDelete(rtn)
	return cachedData
}
