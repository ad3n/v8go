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
	"sync"
	"unsafe"
)

type Isolate struct {
	ptr C.IsolatePtr

	cbMutex sync.RWMutex
	cbSeq   int
	cbs     map[int]FunctionCallbackWithError

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
