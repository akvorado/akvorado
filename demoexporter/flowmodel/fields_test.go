// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flowmodel

import (
	"testing"
	"time"

	"akvorado/common/helpers"
)

func TestSizeBucket(t *testing.T) {
	cases := []struct {
		Input    uint64
		Expected int32
	}{
		{0, 0}, {63, 0}, {64, 1}, {127, 1}, {128, 2},
		{1500, 7}, {1501, 8}, {65535, 15}, {65536, 16}, {1 << 20, 16},
	}
	for _, tc := range cases {
		if diff := helpers.Diff(SizeBucket(tc.Input), tc.Expected); diff != "" {
			t.Errorf("SizeBucket(%d) (-got, +want):\n%s", tc.Input, diff)
		}
	}
	// Bucket ranges should partition the whole space.
	for bucket := range int32(SizeBucketCount) {
		lo, hi := sizeBucketRange(bucket)
		for _, input := range []uint64{lo, hi} {
			if diff := helpers.Diff(SizeBucket(input), bucket); diff != "" {
				t.Errorf("SizeBucket(%d) (-got, +want):\n%s", input, diff)
			}
		}
	}
}

func TestPacketBucket(t *testing.T) {
	cases := []struct {
		Input    uint64
		Expected int32
	}{
		{1, 0}, {2, 1}, {3, 2}, {4, 2}, {5, 3}, {1024, 10}, {1025, 11}, {1 << 20, 11},
	}
	for _, tc := range cases {
		if diff := helpers.Diff(PacketBucket(tc.Input), tc.Expected); diff != "" {
			t.Errorf("PacketBucket(%d) (-got, +want):\n%s", tc.Input, diff)
		}
	}
	for bucket := range int32(PacketBucketCount) {
		lo, hi := packetBucketRange(bucket)
		for _, input := range []uint64{lo, hi} {
			got := PacketBucket(input)
			want := bucket
			if diff := helpers.Diff(got, want); diff != "" {
				t.Errorf("PacketBucket(%d) (-got, +want):\n%s", input, diff)
			}
		}
	}
}

func TestHourOfWeek(t *testing.T) {
	cases := []struct {
		Input    time.Time
		Expected int
	}{
		// A Sunday at midnight UTC.
		{time.Date(2022, 3, 20, 0, 0, 0, 0, time.UTC), 0},
		// A Friday at 15:00 UTC.
		{time.Date(2022, 3, 18, 15, 0, 0, 0, time.UTC), 5*24 + 15},
		// A Saturday at 23:00 UTC.
		{time.Date(2022, 3, 19, 23, 0, 0, 0, time.UTC), 6*24 + 23},
	}
	for _, tc := range cases {
		if diff := helpers.Diff(HourOfWeek(tc.Input), tc.Expected); diff != "" {
			t.Errorf("HourOfWeek(%s) (-got, +want):\n%s", tc.Input, diff)
		}
	}
}

func TestPortIndex(t *testing.T) {
	ports := append(DefaultPortRanges(),
		PortRange{Start: 443, End: 443},
		PortRange{Start: 53, End: 53})
	idx := NewPortIndex(ports)
	cases := []struct {
		Input    uint16
		Expected int32
	}{
		{0, 0},
		{80, 1},    // in 1-1023
		{2000, 2},  // in 1024-4095
		{33000, 5}, // in 32768-49151
		{50000, 6}, // in 49152-65535
		{443, 7},
		{53, 8},
	}
	for _, tc := range cases {
		if diff := helpers.Diff(idx.Token(tc.Input), tc.Expected); diff != "" {
			t.Errorf("Token(%d) (-got, +want):\n%s", tc.Input, diff)
		}
	}
}
