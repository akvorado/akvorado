// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	ipfixVersion      = 10
	ipfixHeaderLength = 16
)

type netflow struct{}

func (netflow) Name() string { return "netflow" }

// Split splits a stream of IPFIX messages using the length from the header.
// NetFlow v9 has no length in its header and is not supported.
func (netflow) Split(data []byte, atEOF bool) (int, []byte, error) {
	if len(data) < 4 {
		if atEOF && len(data) > 0 {
			return 0, nil, io.ErrUnexpectedEOF
		}
		return 0, nil, nil
	}
	version := binary.BigEndian.Uint16(data[0:2])
	length := int(binary.BigEndian.Uint16(data[2:4]))
	if version != ipfixVersion {
		return 0, nil, fmt.Errorf("unexpected IPFIX version %d", version)
	}
	if length < ipfixHeaderLength {
		return 0, nil, fmt.Errorf("invalid IPFIX message length %d", length)
	}
	if len(data) < length {
		if atEOF {
			return 0, nil, io.ErrUnexpectedEOF
		}
		return 0, nil, nil
	}
	return length, data[:length], nil
}
