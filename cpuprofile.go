// Copyright 2021 the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

/*
#include "v8go.h"
#include "errors.h"
*/
import "C"
import "time"

type CPUProfile struct {
	p *C.CPUProfile

	title string

	root *CPUProfileNode

	startTimeOffset time.Duration

	endTimeOffset time.Duration
}

func (c *CPUProfile) GetTitle() string {
	return c.title
}

func (c *CPUProfile) GetTopDownRoot() *CPUProfileNode {
	return c.root
}

func (c *CPUProfile) GetDuration() time.Duration {
	return c.endTimeOffset - c.startTimeOffset
}

func (c *CPUProfile) Delete() {
	if c.p == nil {
		return
	}

	C.CPUProfileDelete(c.p)
	c.p = nil
}
