// Copyright 2021 the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

type CPUProfileNode struct {
	parent *CPUProfileNode

	scriptResourceName string

	functionName string

	bailoutReason string

	children []*CPUProfileNode

	nodeId int

	scriptId int

	lineNumber int

	columnNumber int

	hitCount int
}

func (c *CPUProfileNode) GetNodeId() int {
	return c.nodeId
}

func (c *CPUProfileNode) GetScriptId() int {
	return c.scriptId
}

func (c *CPUProfileNode) GetFunctionName() string {
	return c.functionName
}

func (c *CPUProfileNode) GetScriptResourceName() string {
	return c.scriptResourceName
}

func (c *CPUProfileNode) GetLineNumber() int {
	return c.lineNumber
}

func (c *CPUProfileNode) GetColumnNumber() int {
	return c.columnNumber
}

func (c *CPUProfileNode) GetHitCount() int {
	return c.hitCount
}

func (c *CPUProfileNode) GetBailoutReason() string {
	return c.bailoutReason
}

func (c *CPUProfileNode) GetParent() *CPUProfileNode {
	return c.parent
}

func (c *CPUProfileNode) GetChildrenCount() int {
	return len(c.children)
}

func (c *CPUProfileNode) GetChild(index int) *CPUProfileNode {
	return c.children[index]
}
