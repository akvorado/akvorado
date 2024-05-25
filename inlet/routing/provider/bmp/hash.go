// SPDX-FileCopyrightText: 2019 The Go Authors
// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileComment: This is an excerpt from src/hash/maphash/maphash.go

package bmp

import "unsafe"

// In Go 1.23, we may not be able to do that. Check here what they do:
// https://github.com/dgraph-io/ristretto/blob/master/z/rtutil.go#L42-L44

//go:linkname memhash runtime.memhash
//go:noescape
func memhash(p unsafe.Pointer, seed, s uintptr) uintptr

func rthash(ptr *byte, len int, seed uint64) uint64 {
	if len == 0 {
		return seed
	}
	// The runtime hasher only works on uintptr. For 64-bit
	// architectures, we use the hasher directly. Otherwise,
	// we use two parallel hashers on the lower and upper 32 bits.
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return uint64(memhash(unsafe.Pointer(ptr), uintptr(seed), uintptr(len)))
	}
	lo := memhash(unsafe.Pointer(ptr), uintptr(seed), uintptr(len))
	hi := memhash(unsafe.Pointer(ptr), uintptr(seed>>32), uintptr(len))
	return uint64(hi)<<32 | uint64(lo)
}
