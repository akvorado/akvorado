// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build 386 || amd64 || amd64p32 || arm || arm64 || loong64 || mipsle || mis64le || mips64p32le || ppc64le || riscv || riscv64 || wasm

package helpers

import "encoding/binary"

// NativeEndian implements binary native byte order.
var NativeEndian = binary.LittleEndian
