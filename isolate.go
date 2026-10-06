// Copyright 2019 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include <stdlib.h>
// #include "isolate.h"
import "C"

import (
	"io"
	"runtime"
	"runtime/cgo"
	"strconv"
	"sync"
	"unsafe"
)

// PromiseRejectEvent is the kind of event passed to a
// [PromiseRejectedCallback]. The values mirror v8::PromiseRejectEvent.
//
// See also: https://v8.github.io/api/head/namespacev8.html
type PromiseRejectEvent uint8

const (
	// PromiseRejectWithNoHandler is sent when a promise is rejected, and
	// it has no rejection handler.
	PromiseRejectWithNoHandler PromiseRejectEvent = 0

	// PromiseHandlerAddedAfterReject is sent when a rejection handler is
	// added to a promise that has already been rejected. E.g., the
	// following sends a PromiseRejectWithNoHandler event, followed by a
	// PromiseHandlerAddedAfterReject event:
	//
	// 	Promise.reject("dummy").catch(e => {})
	PromiseHandlerAddedAfterReject PromiseRejectEvent = 1
)

func (e PromiseRejectEvent) String() string {
	switch e {
	case PromiseRejectWithNoHandler:
		return "PromiseRejectWithNoHandler"
	case PromiseHandlerAddedAfterReject:
		return "PromiseHandlerAddedAfterReject"
	default:
		return "PromiseRejectEvent(" + strconv.Itoa(int(e)) + ")"
	}
}

// Isolate is a JavaScript VM instance with its own heap and
// garbage collector. Most applications will create one isolate
// with many V8 contexts for execution.
type Isolate struct {
	ptr C.IsolatePtr

	cbMutex sync.RWMutex
	cbSeq   int
	cbs     map[int]FunctionCallbackWithError

	// promiseRejectedCallback is called from goPromiseRejectedCallback.
	promiseRejectedCallback PromiseRejectedCallback

	null      *Value
	undefined *Value
}

type HeapStatistics struct {
	TotalHeapSize            uint64
	TotalHeapSizeExecutable  uint64
	TotalPhysicalSize        uint64
	TotalAvailableSize       uint64
	UsedHeapSize             uint64
	HeapSizeLimit            uint64
	MallocedMemory           uint64
	ExternalMemory           uint64
	PeakMallocedMemory       uint64
	NumberOfNativeContexts   uint64
	NumberOfDetachedContexts uint64
}

type resourceConstraints struct {
	InitialHeapSizeInBytes uint64
	MaxHeapSizeInBytes     uint64
}

type IsolateOption func(*isolateConfig)

type isolateConfig struct {
	resourceConstraints *resourceConstraints
	exceptionMessages   bool
}

func WithExceptionMessages() IsolateOption {
	return func(config *isolateConfig) {
		config.exceptionMessages = true
	}
}

func WithResourceConstraints(initialHeapSizeInBytes, maxHeapSizeInBytes uint64) IsolateOption {
	return func(config *isolateConfig) {
		config.resourceConstraints = &resourceConstraints{
			InitialHeapSizeInBytes: initialHeapSizeInBytes,
			MaxHeapSizeInBytes:     maxHeapSizeInBytes,
		}
	}
}

func NewIsolate(opts ...IsolateOption) *Isolate {
	initializeIfNecessary()

	config := &isolateConfig{}

	for _, opt := range opts {
		opt(config)
	}

	var cConstraints C.IsolateConstraintsPtr
	if config.resourceConstraints != nil {
		cConstraints = &C.IsolateConstraints{
			initial_heap_size_in_bytes: C.size_t(config.resourceConstraints.InitialHeapSizeInBytes),
			maximum_heap_size_in_bytes: C.size_t(config.resourceConstraints.MaxHeapSizeInBytes),
		}
	}

	iso := &Isolate{
		ptr: C.NewIsolate(cConstraints),
		cbs: make(map[int]FunctionCallbackWithError),
	}

	if config.exceptionMessages {
		C.IsolateSetExceptionMessages(iso.ptr, 1)
	}

	iso.null = newValueNull(iso)
	iso.undefined = newValueUndefined(iso)
	return iso
}

func (i *Isolate) TerminateExecution() {
	C.IsolateTerminateExecution(i.ptr)
}

func (i *Isolate) IsExecutionTerminating() bool {
	return C.IsolateIsExecutionTerminating(i.ptr) == 1
}

type CompileOptions struct {
	CachedData *CompilerCachedData

	Mode CompileMode
}

func (i *Isolate) CompileUnboundScript(
	source, origin string,
	opts CompileOptions,
) (*UnboundScript, error) {
	cSource := C.CString(source)
	cOrigin := C.CString(origin)
	defer C.free(unsafe.Pointer(cSource))
	defer C.free(unsafe.Pointer(cOrigin))

	var cOptions C.CompileOptions
	cOptions.compileOption = C.int(opts.Mode)
	if opts.CachedData != nil {
		if opts.Mode != 0 {
			panic("On CompileOptions, Mode and CachedData can't both be set")
		}

		cOptions.compileOption = C.ScriptCompilerConsumeCodeCache
		cOptions.cachedData.length = C.int(len(opts.CachedData.Bytes))
		if len(opts.CachedData.Bytes) > 0 {
			cOptions.cachedData.data = (*C.uchar)(unsafe.Pointer(&opts.CachedData.Bytes[0]))
		}
	}

	rtn := C.IsolateCompileUnboundScript(i.ptr, cSource, cOrigin, cOptions)
	runtime.KeepAlive(opts.CachedData)
	if rtn.ptr == nil {
		return nil, newJSError(rtn.error)
	}

	if opts.CachedData != nil {
		opts.CachedData.Rejected = int(rtn.cachedDataRejected) == 1
	}

	return &UnboundScript{
		ptr: rtn.ptr,
		iso: i,
	}, nil
}

func (i *Isolate) GetHeapStatistics() HeapStatistics {
	hs := C.IsolationGetHeapStatistics(i.ptr)

	return HeapStatistics{
		TotalHeapSize:            uint64(hs.total_heap_size),
		TotalHeapSizeExecutable:  uint64(hs.total_heap_size_executable),
		TotalPhysicalSize:        uint64(hs.total_physical_size),
		TotalAvailableSize:       uint64(hs.total_available_size),
		UsedHeapSize:             uint64(hs.used_heap_size),
		HeapSizeLimit:            uint64(hs.heap_size_limit),
		MallocedMemory:           uint64(hs.malloced_memory),
		ExternalMemory:           uint64(hs.external_memory),
		PeakMallocedMemory:       uint64(hs.peak_malloced_memory),
		NumberOfNativeContexts:   uint64(hs.number_of_native_contexts),
		NumberOfDetachedContexts: uint64(hs.number_of_detached_contexts),
	}
}

func (i *Isolate) LowMemoryNotification() {
	C.IsolateLowMemoryNotification(i.ptr)
}

func (i *Isolate) WriteHeapSnapshot(w io.Writer) error {
	hw := heapSnapshotWriter{w: w}

	h := cgo.NewHandle(&hw)
	defer h.Delete()

	C.IsolateWriteHeapSnapshot(i.ptr, C.uintptr_t(h))
	return hw.err
}

type heapSnapshotWriter struct {
	w   io.Writer
	err error
}

//export goWriteHeapSnapshotChunk
func goWriteHeapSnapshotChunk(writerRef C.uintptr_t, data *C.char, size C.int) C.int {
	hw := cgo.Handle(writerRef).Value().(*heapSnapshotWriter)

	n, err := hw.w.Write(unsafe.Slice((*byte)(unsafe.Pointer(data)), int(size)))
	if err == nil && n < int(size) {
		err = io.ErrShortWrite
	}

	if err != nil {
		hw.err = err
		return 0
	}

	return 1
}

func (i *Isolate) Dispose() {
	if i.ptr == nil {
		return
	}

	C.IsolateDispose(i.ptr)
	i.ptr = nil
}

func (i *Isolate) ThrowException(value *Value) *Value {
	if i.ptr == nil {
		panic("Isolate has been disposed")
	}

	return &Value{
		ptr: C.IsolateThrowException(i.ptr, value.ptr),
	}
}

func (i *Isolate) Close() {
	i.Dispose()
}

func (i *Isolate) apply(opts *contextOptions) {
	opts.iso = i
}

func (i *Isolate) registerCallback(cb FunctionCallbackWithError) int {
	i.cbMutex.Lock()
	i.cbSeq++
	ref := i.cbSeq
	i.cbs[ref] = cb
	i.cbMutex.Unlock()
	return ref
}

func (i *Isolate) getCallback(ref int) FunctionCallbackWithError {
	i.cbMutex.RLock()
	defer i.cbMutex.RUnlock()
	return i.cbs[ref]
}

// PromiseRejectMessage is passed to a [PromiseRejectedCallback]. The fields
// reflect v8::PromiseRejectMessage.
//
// See also: https://v8.github.io/api/head/classv8_1_1PromiseRejectMessage.html
type PromiseRejectMessage struct {
	// Context is the context the promise was created in.
	Context *Context
	Promise *Promise
	Event   PromiseRejectEvent
	// Value is the rejection value. It is nil for
	// PromiseHandlerAddedAfterReject.
	Value *Value
}

// PromiseRejectedCallback is called with promise rejection events. See
// [Isolate.SetPromiseRejectedCallback].
type PromiseRejectedCallback func(PromiseRejectMessage)

// SetPromiseRejectedCallback sets the callback to be called for promise
// rejection events. This includes rejections that happen while V8 runs
// microtasks after a script has finished. A nil callback removes it. An
// Isolate has at most one callback, so this replaces any previous one.
//
// The callback runs synchronously inside V8, and must not run JavaScript
// in the context of the promise.
//
// The callback is not called for promises created in a closed Context,
// since their values can't be tracked. This can happen if a closed
// context's promise is still reachable from another context.
func (i *Isolate) SetPromiseRejectedCallback(cb PromiseRejectedCallback) {
	if i.ptr == nil {
		panic("Isolate has been disposed")
	}
	i.promiseRejectedCallback = cb
	C.IsolateSetPromiseRejectedCallback(i.ptr, C.bool(cb != nil))
}

// goPromiseRejectedCallback is the V8 promise reject callback set by
// SetPromiseRejectedCallback. value is nil if the event has no value.
//
//export goPromiseRejectedCallback
func goPromiseRejectedCallback(ctxref int, event C.int, promise C.ValuePtr, value C.ValuePtr) {
	ctx := getContext(ctxref)
	cb := ctx.iso.promiseRejectedCallback
	if cb == nil {
		return
	}

	msg := PromiseRejectMessage{
		Context: ctx,
		Promise: &Promise{&Object{&Value{ptr: promise, ctx: ctx}}},
		Event:   PromiseRejectEvent(event),
	}
	if value != nil {
		msg.Value = &Value{ptr: value, ctx: ctx}
	}
	cb(msg)
}
