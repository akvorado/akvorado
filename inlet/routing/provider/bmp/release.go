// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build release

package bmp

import (
	"math/rand"
)

const rtaHashMask = 0xffffffffffffffff

var rtaHashSeed uint64

func init() {
	rtaHashSeed = rand.Uint64()
}
