// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flowmodel

import (
	"time"

	"akvorado/common/schema"
)

// Each flow is described by a fixed sequence of tokens. The first
// field only conditions the model, the other fields are predicted one
// after the other.
const (
	FieldHourOfWeek = iota
	FieldDirection
	FieldLocalBoundary
	FieldProtoEtype
	FieldRemotePrefix
	FieldLocalPrefix
	FieldSrcPort
	FieldDstPort
	FieldSizeBucket
	FieldPacketBucket
	// FieldCount is the number of fields describing one flow.
	FieldCount
)

// Direction tells if a flow goes to a customer or comes from a customer.
type Direction int

const (
	// DirectionInbound is traffic from the Internet to a customer.
	DirectionInbound Direction = iota
	// DirectionOutbound is traffic from a customer to the Internet.
	DirectionOutbound
)

const (
	hourOfWeekCount = 168
	directionCount  = 2
	boundaryCount   = 3
)

// HourOfWeek returns the hour-of-week bucket for a time, in UTC.
func HourOfWeek(t time.Time) int {
	t = t.UTC()
	return int(t.Weekday())*24 + t.Hour()
}

// sizeBucketBounds are the bounds of the PacketSizeBucket column.
var sizeBucketBounds = schema.PacketSizeBucketBounds

// SizeBucketCount is the number of packet size buckets.
const SizeBucketCount = len(sizeBucketBounds) + 1

// SizeBucket returns the bucket for a bytes-per-packet value.
func SizeBucket(bytesPerPacket uint64) int32 {
	for i, bound := range sizeBucketBounds {
		if bytesPerPacket < bound {
			return int32(i)
		}
	}
	return int32(len(sizeBucketBounds))
}

// sizeBucketRange returns the inclusive bounds of a packet size bucket.
func sizeBucketRange(bucket int32) (lo, hi uint64) {
	if bucket > 0 {
		lo = sizeBucketBounds[bucket-1]
	}
	if int(bucket) < len(sizeBucketBounds) {
		hi = sizeBucketBounds[bucket] - 1
	} else {
		hi = sizeBucketBounds[len(sizeBucketBounds)-1]
	}
	return lo, hi
}

// packetBucketBounds are the inclusive upper bounds of the
// packets-per-flow buckets. A last bucket catches everything above
// the last bound.
var packetBucketBounds = [...]uint64{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024}

// PacketBucketCount is the number of packets-per-flow buckets.
const PacketBucketCount = len(packetBucketBounds) + 1

// PacketBucket returns the bucket for a packets-per-flow value.
func PacketBucket(packets uint64) int32 {
	for i, bound := range packetBucketBounds {
		if packets <= bound {
			return int32(i)
		}
	}
	return int32(len(packetBucketBounds))
}

// packetBucketRange returns the inclusive bounds of a packets-per-flow bucket.
func packetBucketRange(bucket int32) (lo, hi uint64) {
	lo = 1
	if bucket > 0 {
		lo = packetBucketBounds[bucket-1] + 1
	}
	if int(bucket) < len(packetBucketBounds) {
		hi = packetBucketBounds[bucket]
	} else {
		hi = 2 * packetBucketBounds[len(packetBucketBounds)-1]
	}
	return lo, hi
}

// DefaultPortRanges are the port range buckets always present at the start of
// the port vocabulary. Ports popular enough to get their own entry are appended
// after them.
func DefaultPortRanges() []PortRange {
	return []PortRange{
		{Start: 0, End: 0},
		{Start: 1, End: 1023},
		{Start: 1024, End: 4095},
		{Start: 4096, End: 16383},
		{Start: 16384, End: 32767},
		{Start: 32768, End: 49151},
		{Start: 49152, End: 65535},
	}
}

// PortIndex maps a port value to its token in the port vocabulary.
type PortIndex struct {
	exact  map[uint16]int32
	ranges []int32 // token for each range entry, in order
	meta   []PortRange
}

// NewPortIndex builds an index for the given port vocabulary.
func NewPortIndex(ports []PortRange) *PortIndex {
	idx := PortIndex{
		exact: make(map[uint16]int32),
		meta:  ports,
	}
	for i, p := range ports {
		if p.Start == p.End {
			idx.exact[p.Start] = int32(i)
		} else {
			idx.ranges = append(idx.ranges, int32(i))
		}
	}
	return &idx
}

// Token returns the token for a port. Exact entries win over range entries. It
// returns 0 when nothing matches, which cannot happen when the vocabulary
// contains the default port ranges.
func (idx *PortIndex) Token(port uint16) int32 {
	if token, ok := idx.exact[port]; ok {
		return token
	}
	for _, token := range idx.ranges {
		p := idx.meta[token]
		if port >= p.Start && port <= p.End {
			return token
		}
	}
	return 0
}
