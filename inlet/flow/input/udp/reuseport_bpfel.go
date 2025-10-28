// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build (386 || amd64 || arm || arm64 || loong64 || mips64le || mipsle || ppc64le || riscv64 || wasm) && linux

package udp

import (
	_ "embed"
)

//go:embed reuseport_bpfel.o
var bpfBytes []byte
