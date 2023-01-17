// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestMaxSizeVarint(t *testing.T) {
	got := protowire.SizeVarint(^uint64(0))
	if got != maxSizeVarint {
		t.Fatalf("maximum size for varint is %d, not %d", got, maxSizeVarint)
	}
}
