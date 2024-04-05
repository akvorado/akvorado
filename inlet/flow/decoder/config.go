// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

import (
	"errors"

	"akvorado/common/helpers/bimap"
)

// TimestampSource defines the method to use to extract the TimeReceived for the flows
type TimestampSource uint

const (
	// TimestampSourceUDP tells the decoder to use the kernel time at which
	// the UDP packet was received
	TimestampSourceUDP TimestampSource = iota
	// TimestampSourceNetflowPacket tells the decoder to use the timestamp
	// from the router in the netflow packet
	TimestampSourceNetflowPacket
	// TimestampSourceNetflowFirstSwitched tells the decoder to use the timestamp
	// from each flow "FIRST_SWITCHED" field
	TimestampSourceNetflowFirstSwitched
)

var (
	timestampSourceMap = bimap.New(map[TimestampSource]string{
		TimestampSourceUDP:                  "udp",
		TimestampSourceNetflowPacket:        "netflow-packet",
		TimestampSourceNetflowFirstSwitched: "netflow-first-switched",
	})
	errUnknownTimestampSource = errors.New("unknown TimestampSource")
)

// MarshalText turns an interface boundary to text
func (ib TimestampSource) MarshalText() ([]byte, error) {
	got, ok := timestampSourceMap.LoadValue(ib)
	if ok {
		return []byte(got), nil
	}
	return nil, errUnknownTimestampSource
}

// String turns an interface boundary to string
func (ib TimestampSource) String() string {
	got, _ := timestampSourceMap.LoadValue(ib)
	return got
}

// UnmarshalText provides an interface boundary from text
func (ib *TimestampSource) UnmarshalText(input []byte) error {
	if len(input) == 0 {
		*ib = TimestampSourceUDP
		return nil
	}
	got, ok := timestampSourceMap.LoadKey(string(input))
	if ok {
		*ib = got
		return nil
	}
	return errUnknownTimestampSource
}
