// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

// EncodeMessage will marshal a protobuf message using the length-prefixed
// representation.
func (m *FlowMessage) EncodeMessage(buf []byte) ([]byte, error) {
	messageSize := m.SizeVT()
	buf = buf[:0]
	buf = protowire.AppendVarint(buf, uint64(messageSize))
	prefixSize := len(buf)
	totalSize := prefixSize + messageSize
	if cap(buf) < totalSize {
		newBuf := make([]byte, totalSize)
		copy(newBuf, buf)
		buf = newBuf
	} else {
		buf = buf[:totalSize]
	}
	n, err := m.MarshalToSizedBufferVT(buf[prefixSize:])
	if n != messageSize {
		return buf, fmt.Errorf("incorrect size for proto buffer (%d vs %d)", n, messageSize)
	}
	return buf, err
}
