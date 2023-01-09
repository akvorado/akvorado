// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import "google.golang.org/protobuf/encoding/protowire"

// ProtoMessage is any object implementing size and marshal using VT protobuf.
type ProtoMessage interface {
	SizeVT() int
	MarshalToSizedBufferVT([]byte) (int, error)
}

// MarshalProto will marshal a protobuf message using the length-prefixed
// representation.
func MarshalProto(buf []byte, msg ProtoMessage) ([]byte, error) {
	messageSize := msg.SizeVT()
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
	_, err := msg.MarshalToSizedBufferVT(buf[prefixSize:])
	return buf, err
}
