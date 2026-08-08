// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package networks

import (
	"fmt"
	"math/rand/v2"
	"net/netip"
	"runtime"
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// randomAddr builds a random IPv4 address.
func randomAddr(prng *rand.Rand) netip.Addr {
	var addr [4]byte
	for i := range addr {
		addr[i] = uint8(prng.UintN(256))
	}
	return netip.AddrFrom4(addr)
}

// randomNetworks builds a list of prefixes sharing a limited number of
// distinct attribute sets, like a GeoIP database does.
func randomNetworks(prng *rand.Rand, prefixes, distinct int) []externalNetworkAttributes {
	attributes := make([]NetworkAttributes, distinct)
	for i := range attributes {
		attributes[i] = NetworkAttributes{
			ASN:     uint32(i + 1),
			Country: fmt.Sprintf("C%d", i%250),
			State:   fmt.Sprintf("state-%d", i%1000),
			City:    fmt.Sprintf("city-%d", i),
		}
	}
	result := make([]externalNetworkAttributes, prefixes)
	for i := range result {
		result[i] = externalNetworkAttributes{
			Prefix:            netip.PrefixFrom(randomAddr(prng), 8+prng.IntN(17)).Masked(),
			NetworkAttributes: attributes[prng.IntN(distinct)],
		}
	}
	return result
}

// newTestComponent builds a component fed by a single remote source.
func newTestComponent(t testing.TB, prefixes, distinct int) *Component {
	t.Helper()
	c, err := New(reporter.NewMock(t), DefaultConfiguration(),
		Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	c.networkSources["generated"] = randomNetworks(
		rand.New(rand.NewPCG(10, 10)), prefixes, distinct)
	return c
}

func BenchmarkRebuild(b *testing.B) {
	for _, prefixes := range []int{100_000, 1_000_000} {
		for _, distinct := range []int{1_000, 100_000} {
			b.Run(fmt.Sprintf("%d prefixes, %d distinct", prefixes, distinct), func(b *testing.B) {
				c := newTestComponent(b, prefixes, distinct)

				// The source is already built, only the tree and the pool are
				// accounted by the difference.
				var startMem, endMem runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&startMem)
				for b.Loop() {
					c.rebuild()
				}
				runtime.GC()
				runtime.ReadMemStats(&endMem)

				networks := c.networks.Load()
				prefixes := float64(networks.prefixes.Size())
				b.ReportMetric(float64(endMem.HeapAlloc-startMem.HeapAlloc)/prefixes, "bytes/prefix")
				// What the memory_bytes metric reports, to tell how far the
				// estimation based on the heap profile is.
				b.ReportMetric(float64(measureMemory())/prefixes, "profiled-bytes/prefix")
				b.ReportMetric(float64(networks.pool.Len()), "interned")
			})
		}
	}
}

func BenchmarkLookup(b *testing.B) {
	c := newTestComponent(b, 1_000_000, 100_000)
	c.rebuild()

	prng := rand.New(rand.NewPCG(20, 20))
	addresses := make([]netip.Addr, 1024)
	for i := range addresses {
		addresses[i] = helpers.AddrTo6(randomAddr(prng))
	}

	i := 0
	for b.Loop() {
		c.Lookup(addresses[i%len(addresses)])
		i++
	}
}

func BenchmarkMeasureMemory(b *testing.B) {
	c := newTestComponent(b, 1_000_000, 100_000)
	c.rebuild()
	for b.Loop() {
		measureMemory()
	}
}
