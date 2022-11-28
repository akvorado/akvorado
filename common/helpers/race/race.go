// SPDX-FileCopyrightText: 2009 The Go Authors
// SPDX-License-Identifier: BSD-3-Clause

//go:build race

package race

import (
	"runtime"
	"unsafe"
)

// Enabled reports if the race detector is enabled.
const Enabled = true

// Disable has the same semantics as runtime.RaceDisable.
func Disable() {
	runtime.RaceDisable()
}

// Enable has the same semantics as runtime.RaceEnable.
func Enable() {
	runtime.RaceEnable()
}

// Acquire has the same semantics as runtime.RaceAcquire.
func Acquire(addr unsafe.Pointer) {
	runtime.RaceAcquire(addr)
}

// Release has the same semantics as runtime.RaceRelease.
func Release(addr unsafe.Pointer) {
	runtime.RaceRelease(addr)
}

// ReleaseMerge has the same semantics as runtime.RaceReleaseMerge.
func ReleaseMerge(addr unsafe.Pointer) {
	runtime.RaceReleaseMerge(addr)
}
