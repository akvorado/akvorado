// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build release

package bmp

import (
	"math/rand"
	"time"
)

const rtaHashMask = 0xffffffffffffffff

var rtaHashSeed uint64

func init() {
	rand.Seed(time.Now().UnixMicro())
	rtaHashSeed = rand.Uint64()
}
