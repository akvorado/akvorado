// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build (mips || mips64 || ppc64 || s390x) && linux

package udp

import (
	_ "embed"
)

//go:embed reuseport_bpfeb.o
var bpfBytes []byte
