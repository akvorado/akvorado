// SPDX-FileCopyrightText: 2009 The Go Authors
// SPDX-License-Identifier: BSD-3-Clause

//go:build race

package helpers

import (
	"runtime"
	"unsafe"
)

// RaceEnabled reports if the race detector is enabled.
const RaceEnabled = true

// RaceDisable has the same semantics as runtime.RaceDisable.
func RaceDisable() {
	runtime.RaceDisable()
}

// RaceEnable has the same semantics as runtime.RaceEnable.
func RaceEnable() {
	runtime.RaceEnable()
}

// RaceAcquire has the same semantics as runtime.RaceAcquire.
func RaceAcquire(addr unsafe.Pointer) {
	runtime.RaceAcquire(addr)
}

// RaceRelease has the same semantics as runtime.RaceRelease.
func RaceRelease(addr unsafe.Pointer) {
	runtime.RaceRelease(addr)
}

// RaceReleaseMerge has the same semantics as runtime.RaceReleaseMerge.
func RaceReleaseMerge(addr unsafe.Pointer) {
	runtime.RaceReleaseMerge(addr)
}
