// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import "encoding/binary"

// NativeEndian implements binary native byte order.
var NativeEndian = binary.LittleEndian
