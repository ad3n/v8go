// Copyright 2019 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include "v8go.h"
// #include <stdlib.h>
import "C"
import (
	"strings"
	"sync"
	"unsafe"
)

func Version() string {
	return C.GoString(C.Version())
}

func SetFlags(flags ...string) {
	cflags := C.CString(strings.Join(flags, " "))
	C.SetFlags(cflags)
	C.free(unsafe.Pointer(cflags))
}

func initializeIfNecessary() {
	v8once.Do(func() {
		cflags := C.CString("--no-freeze_flags_after_init")
		defer C.free(unsafe.Pointer(cflags))
		C.SetFlags(cflags)
		C.Init()
	})
}

var v8once sync.Once
