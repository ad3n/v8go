// Copyright 2021 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include <stdlib.h>
// #include "object.h"
import "C"
import (
	"fmt"
	"math/big"
	"unsafe"
)

type Object struct {
	*Value
}

func (o *Object) MethodCall(methodName string, args ...Valuer) (*Value, error) {
	ckey := C.CString(methodName)
	defer C.free(unsafe.Pointer(ckey))

	getRtn := C.ObjectGet(o.ptr, ckey)
	prop, err := valueResult(o.ctx, getRtn)
	if err != nil {
		return nil, err
	}

	fn, err := prop.AsFunction()
	if err != nil {
		return nil, err
	}

	return fn.Call(o, args...)
}

func coerceValue(iso *Isolate, val any) (*Value, error) {
	switch v := val.(type) {
	case string, int32, uint32, int64, uint64, float64, bool, *big.Int:

		value, _ := NewValue(iso, v)
		return value, nil
	case Valuer:
		return v.value(), nil
	default:
		return nil, fmt.Errorf("v8go: unsupported object property type `%T`", v)
	}
}

func (o *Object) Set(key string, val any) error {
	value, err := coerceValue(o.ctx.iso, val)
	if err != nil {
		return err
	}

	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))
	C.ObjectSet(o.ptr, ckey, value.ptr)
	return nil
}

func (o *Object) SetSymbol(key *Symbol, val any) error {
	value, err := coerceValue(o.ctx.iso, val)
	if err != nil {
		return err
	}

	C.ObjectSetAnyKey(o.ptr, key.ptr, value.ptr)
	return nil
}

func (o *Object) SetIdx(idx uint32, val any) error {
	value, err := coerceValue(o.ctx.iso, val)
	if err != nil {
		return err
	}

	C.ObjectSetIdx(o.ptr, C.uint32_t(idx), value.ptr)

	return nil
}

func (o *Object) SetInternalField(idx uint32, val any) error {
	value, err := coerceValue(o.ctx.iso, val)

	if err != nil {
		return err
	}

	inserted := C.ObjectSetInternalField(o.ptr, C.int(idx), value.ptr)

	if inserted == 0 {
		panic(fmt.Errorf("index out of range [%v] with length %v", idx, o.InternalFieldCount()))
	}

	return nil
}

func (o *Object) InternalFieldCount() uint32 {
	count := C.ObjectInternalFieldCount(o.ptr)
	return uint32(count)
}

func (o *Object) Get(key string) (*Value, error) {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))

	rtn := C.ObjectGet(o.ptr, ckey)
	return valueResult(o.ctx, rtn)
}

func (o *Object) GetSymbol(key *Symbol) (*Value, error) {
	rtn := C.ObjectGetAnyKey(o.ptr, key.ptr)
	return valueResult(o.ctx, rtn)
}

func (o *Object) GetInternalField(idx uint32) *Value {
	rtn := C.ObjectGetInternalField(o.ptr, C.int(idx))
	if rtn.value == nil {
		panic(newJSError(rtn.error))
	}

	return &Value{rtn.value, o.ctx}
}

func (o *Object) GetIdx(idx uint32) (*Value, error) {
	rtn := C.ObjectGetIdx(o.ptr, C.uint32_t(idx))
	return valueResult(o.ctx, rtn)
}

func (o *Object) Has(key string) bool {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))
	return C.ObjectHas(o.ptr, ckey) != 0
}

func (o *Object) HasSymbol(key *Symbol) bool {
	return C.ObjectHasAnyKey(o.ptr, key.ptr) != 0
}

func (o *Object) HasIdx(idx uint32) bool {
	return C.ObjectHasIdx(o.ptr, C.uint32_t(idx)) != 0
}

func (o *Object) Delete(key string) bool {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))
	return C.ObjectDelete(o.ptr, ckey) != 0
}

func (o *Object) DeleteSymbol(key *Symbol) bool {
	return C.ObjectDeleteAnyKey(o.ptr, key.ptr) != 0
}

func (o *Object) DeleteIdx(idx uint32) bool {
	return C.ObjectDeleteIdx(o.ptr, C.uint32_t(idx)) != 0
}
