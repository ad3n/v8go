// Copyright 2019 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include <stdlib.h>
// #include "errors.h"
import "C"
import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var ErrHeapLimitReached = errors.New("heap limit reached")

type JSError struct {
	Message    string
	Location   string
	StackTrace string

	cause error

	message string
}

type Message struct {
	Text string

	ScriptResourceName string

	SourceLine string

	LineNumber int

	StartPosition, EndPosition int

	StartColumn, EndColumn int

	WASMFunctionIndex int

	StackTrace []StackFrame
}

type StackFrame struct {
	ScriptName   string
	FunctionName string

	LineNumber, ColumnNumber int

	IsEval           bool
	IsConstructor    bool
	IsWASM           bool
	IsUserJavaScript bool
}

func newJSError(rtnErr C.RtnError) error {
	err := &JSError{
		Message:    C.GoString(rtnErr.msg),
		Location:   C.GoString(rtnErr.location),
		StackTrace: C.GoString(rtnErr.stack),
	}

	if rtnErr.message != nil {
		err.message = C.GoStringN(rtnErr.message, rtnErr.message_length)
	}

	if rtnErr.heap_limit_reached != 0 {
		err.cause = ErrHeapLimitReached
	}

	C.ErrorRelease(rtnErr)
	return err
}

func (e *JSError) ExceptionMessage() *Message {
	if e.message == "" {
		return nil
	}

	return parseMessage(e.message)
}

func parseMessage(s string) *Message {
	r := messageReader{s: s}

	m := &Message{
		Text:               r.string(),
		ScriptResourceName: r.string(),
		SourceLine:         r.string(),
		LineNumber:         r.int(),
		StartPosition:      r.int(),
		EndPosition:        r.int(),
		StartColumn:        r.int(),
		EndColumn:          r.int(),
		WASMFunctionIndex:  r.int(),
	}

	if n := r.int(); n > 0 {
		m.StackTrace = make([]StackFrame, n)
		for i := range m.StackTrace {
			f := &m.StackTrace[i]
			f.ScriptName = r.string()
			f.FunctionName = r.string()
			f.LineNumber = r.int()
			f.ColumnNumber = r.int()
			flags := r.int()
			f.IsEval = flags&1 != 0
			f.IsConstructor = flags&2 != 0
			f.IsWASM = flags&4 != 0
			f.IsUserJavaScript = flags&8 != 0
		}
	}

	if r.s != "" {
		panic(fmt.Sprintf("v8go: %d trailing bytes in exception message", len(r.s)))
	}

	return m
}

type messageReader struct {
	s string
}

func (r *messageReader) int() int {
	if len(r.s) < 4 {
		panic("v8go: truncated exception message")
	}

	v := int32(binary.LittleEndian.Uint32([]byte(r.s[:4])))
	r.s = r.s[4:]
	return int(v)
}

func (r *messageReader) string() string {
	n := r.int()
	if n < 0 || len(r.s) < n {
		panic("v8go: truncated exception message")
	}

	v := r.s[:n]
	r.s = r.s[n:]
	return v
}

func (e *JSError) Unwrap() error {
	return e.cause
}

func (e *JSError) Error() string {
	return e.Message
}

func (e *JSError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') && e.StackTrace != "" {

			io.WriteString(s, e.StackTrace)

			if e.StackTrace == e.Message && e.Location != "" {
				fmt.Fprintf(s, " (at %s)", e.Location)
			}

			return
		}

		fallthrough
	case 's':
		io.WriteString(s, e.Message)
	case 'q':
		fmt.Fprintf(s, "%q", e.Message)
	}
}
