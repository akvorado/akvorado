// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

// EncodeMessage will marshal a protobuf message using the length-prefixed
// representation.
func (m *FlowMessage) EncodeMessage() ([]byte, error) {
	messageSize := m.SizeVT()
	prefixSize := protowire.SizeVarint(uint64(messageSize))
	totalSize := prefixSize + messageSize
	buf := make([]byte, 0, totalSize)
	buf = protowire.AppendVarint(buf, uint64(messageSize))
	buf = buf[:totalSize]
	n, err := m.MarshalToSizedBufferVT(buf[prefixSize:])
	if n != messageSize {
		return buf, fmt.Errorf("incorrect size for proto buffer (%d vs %d)", n, messageSize)
	}
	return buf, err
}
