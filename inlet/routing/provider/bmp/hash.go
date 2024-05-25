// SPDX-FileCopyrightText: 2019 The Go Authors
// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileComment: This is an excerpt from src/hash/maphash/maphash.go

package bmp

import (
	"hash/maphash"
	"unsafe"
)

var rtaHashSeed = maphash.MakeSeed()

type hash struct {
	h *maphash.Hash
}

func makeHash() hash {
	h := hash{
		h: &maphash.Hash{},
	}
	h.h.SetSeed(rtaHashSeed)
	return h
}

func (h hash) Sum() uint64 {
	return h.h.Sum64()
}

func (h hash) Add(ptr *byte, len int) {
	h.h.Write(unsafe.Slice(ptr, len))
}
