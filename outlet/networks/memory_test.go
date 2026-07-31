// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package networks

import (
	"runtime"
	"testing"

	"akvorado/common/helpers"
)

// TestMeasureMemory checks the tree is accounted for. The runtime only samples
// one allocation every 512 kB by default, therefore the tree has to be large
// enough to be seen at all: the few prefixes used by the other tests would be
// missed entirely and the result would be 0.
func TestMeasureMemory(t *testing.T) {
	if runtime.MemProfileRate == 0 {
		t.Skip("allocation profiling is disabled")
	}
	c := newTestComponent(t, 100_000, 1_000)
	c.rebuild()

	// The heap profile is only refreshed by a garbage collection and lags by up
	// to two cycles.
	runtime.GC()
	runtime.GC()

	got := measureMemory()
	prefixes := c.networks.Load().prefixes.Size()
	// The tree takes around 100 bytes per prefix. The estimation is coarse, only
	// check it is in the right ballpark and not empty.
	if got < int64(prefixes)*10 {
		t.Fatalf("measureMemory() = %d for %d prefixes, expected much more",
			got, prefixes)
	}
}

func TestUnsample(t *testing.T) {
	cases := []struct {
		Name   string
		Count  int64
		Size   int64
		Rate   int64
		Output int64
	}{
		{"nothing sampled", 0, 0, 512 * 1024, 0},
		{"every allocation sampled", 10, 1000, 1, 1000},
		// A single 32 bytes allocation sampled at 512 kB stands for the many
		// others the runtime did not record, so about 512 kB of them.
		{"small allocation", 1, 32, 512 * 1024, 524304},
		// Allocations larger than the rate are always sampled, they stand for
		// themselves.
		{"large allocation", 1, 1024 * 1024, 512 * 1024, 1048576},
	}
	for _, tc := range cases {
		got := unsample(tc.Count, tc.Size, tc.Rate)
		if diff := helpers.Diff(got, tc.Output); diff != "" {
			t.Errorf("unsample(%s) (-got, +want):\n%s", tc.Name, diff)
		}
	}
}
