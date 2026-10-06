// Copyright 2021 the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

/*
#include <stdlib.h>
#include "v8go.h"
*/
import "C"
import (
	"runtime"
	"time"
	"unsafe"
)

type CPUProfiler struct {
	p   *C.CPUProfiler
	iso *Isolate
}

func NewCPUProfiler(iso *Isolate) *CPUProfiler {
	profiler := C.NewCPUProfiler(iso.ptr)
	return &CPUProfiler{
		p:   profiler,
		iso: iso,
	}
}

func (c *CPUProfiler) Dispose() {
	if c.p == nil {
		return
	}

	C.CPUProfilerDispose(c.p)
	c.p = nil
}

func (c *CPUProfiler) Do(title string, fn func()) *CPUProfile {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	c.StartProfiling(title)

	completed := false
	defer func() {
		if !completed {
			c.StopProfiling(title).Delete()
		}
	}()

	fn()
	completed = true

	return c.StopProfiling(title)
}

func (c *CPUProfiler) StartProfiling(title string) {
	if c.p == nil || c.iso.ptr == nil {
		panic("profiler or isolate are nil")
	}

	tstr := C.CString(title)
	defer C.free(unsafe.Pointer(tstr))

	C.CPUProfilerStartProfiling(c.p, tstr)
}

func (c *CPUProfiler) StopProfiling(title string) *CPUProfile {
	if c.p == nil || c.iso.ptr == nil {
		panic("profiler or isolate are nil")
	}

	tstr := C.CString(title)
	defer C.free(unsafe.Pointer(tstr))

	profile := C.CPUProfilerStopProfiling(c.p, tstr)

	return &CPUProfile{
		p:               profile,
		title:           C.GoString(profile.title),
		root:            newCPUProfileNode(profile.root, nil),
		startTimeOffset: time.Duration(profile.startTime) * time.Microsecond,
		endTimeOffset:   time.Duration(profile.endTime) * time.Microsecond,
	}
}

func newCPUProfileNode(node *C.CPUProfileNode, parent *CPUProfileNode) *CPUProfileNode {
	count := countCPUProfileNodes(node)
	nodes := make([]CPUProfileNode, count)
	children := make([]*CPUProfileNode, count-1)
	nextNode, nextChild := 0, 0
	return copyCPUProfileNode(node, parent, nodes, children, &nextNode, &nextChild)
}

func countCPUProfileNodes(node *C.CPUProfileNode) int {
	count := 1
	for _, child := range unsafe.Slice(node.children, int(node.childrenCount)) {
		count += countCPUProfileNodes(child)
	}

	return count
}

func copyCPUProfileNode(node *C.CPUProfileNode, parent *CPUProfileNode, nodes []CPUProfileNode, children []*CPUProfileNode, nextNode, nextChild *int) *CPUProfileNode {
	n := &nodes[*nextNode]
	*nextNode += 1
	*n = CPUProfileNode{
		nodeId:             int(node.nodeId),
		scriptId:           int(node.scriptId),
		scriptResourceName: C.GoString(node.scriptResourceName),
		functionName:       C.GoString(node.functionName),
		lineNumber:         int(node.lineNumber),
		columnNumber:       int(node.columnNumber),
		hitCount:           int(node.hitCount),
		bailoutReason:      C.GoString(node.bailoutReason),
		parent:             parent,
	}

	if node.childrenCount == 0 {
		return n
	}

	end := *nextChild + int(node.childrenCount)
	n.children = children[*nextChild:end:end]
	*nextChild = end
	for i, child := range unsafe.Slice(node.children, int(node.childrenCount)) {
		n.children[i] = copyCPUProfileNode(child, n, nodes, children, nextNode, nextChild)
	}

	return n
}
