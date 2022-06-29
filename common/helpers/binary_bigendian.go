// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build armbe || arm64be || mips || mips64 || mips64p32 || ppc || ppc64 || sparc || sparc64 || s390 || s390x

package helpers

import "encoding/binary"

// NativeEndian implements binary native byte order.
var NativeEndian = binary.BigEndian
