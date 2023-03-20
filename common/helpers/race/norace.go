// SPDX-FileCopyrightText: 2009 The Go Authors
// SPDX-License-Identifier: BSD-3-Clause

//go:build !race

// Package race contains some helpers for race detector.
package race

import "unsafe"

// Enabled reports if the race detector is enabled.
const Enabled = false

// Disable has the same semantics as runtime.Disable.
func Disable() {
}

// Enable has the same semantics as runtime.Enable.
func Enable() {
}

// Acquire has the same semantics as runtime.Acquire.
func Acquire(_ unsafe.Pointer) {
}

// Release has the same semantics as runtime.Release.
func Release(_ unsafe.Pointer) {
}

// ReleaseMerge has the same semantics as runtime.ReleaseMerge.
func ReleaseMerge(_ unsafe.Pointer) {
}
