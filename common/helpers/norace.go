// SPDX-FileCopyrightText: 2009 The Go Authors
// SPDX-License-Identifier: BSD-3-Clause

//go:build !race

package helpers

import "unsafe"

// RaceEnabled reports if the race detector is enabled.
const RaceEnabled = false

// RaceDisable has the same semantics as runtime.RaceDisable.
func RaceDisable() {
}

// RaceEnable has the same semantics as runtime.RaceEnable.
func RaceEnable() {
}

// RaceAcquire has the same semantics as runtime.RaceAcquire.
func RaceAcquire(addr unsafe.Pointer) {
}

// RaceRelease has the same semantics as runtime.RaceRelease.
func RaceRelease(addr unsafe.Pointer) {
}

// RaceReleaseMerge has the same semantics as runtime.RaceReleaseMerge.
func RaceReleaseMerge(addr unsafe.Pointer) {
}
