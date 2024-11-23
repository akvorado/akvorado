// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

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
