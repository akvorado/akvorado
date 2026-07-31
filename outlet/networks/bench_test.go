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

// benchmarkAddr builds a random IPv4 address.
func benchmarkAddr(prng *rand.Rand) netip.Addr {
	var addr [4]byte
	for i := range addr {
		addr[i] = uint8(prng.UintN(256))
	}
	return netip.AddrFrom4(addr)
}

// benchmarkNetworks builds a list of prefixes sharing a limited number of
// distinct attribute sets, like a GeoIP database does.
func benchmarkNetworks(prng *rand.Rand, prefixes, distinct int) []externalNetworkAttributes {
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
			Prefix:            netip.PrefixFrom(benchmarkAddr(prng), 8+prng.IntN(17)).Masked(),
			NetworkAttributes: attributes[prng.IntN(distinct)],
		}
	}
	return result
}

// benchmarkComponent builds a component fed by a single remote source.
func benchmarkComponent(b *testing.B, prefixes, distinct int) *Component {
	b.Helper()
	c, err := New(reporter.NewMock(b), DefaultConfiguration(),
		Dependencies{Daemon: daemon.NewMock(b)})
	if err != nil {
		b.Fatalf("New() error:\n%+v", err)
	}
	c.networkSources["benchmark"] = benchmarkNetworks(
		rand.New(rand.NewPCG(10, 10)), prefixes, distinct)
	return c
}

func BenchmarkRebuild(b *testing.B) {
	for _, prefixes := range []int{100_000, 1_000_000} {
		for _, distinct := range []int{1_000, 100_000} {
			b.Run(fmt.Sprintf("%d prefixes, %d distinct", prefixes, distinct), func(b *testing.B) {
				c := benchmarkComponent(b, prefixes, distinct)

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
				b.ReportMetric(
					float64(endMem.HeapAlloc-startMem.HeapAlloc)/float64(networks.prefixes.Size()),
					"bytes/prefix")
				b.ReportMetric(float64(networks.pool.Len()), "interned")
			})
		}
	}
}

func BenchmarkLookup(b *testing.B) {
	c := benchmarkComponent(b, 1_000_000, 100_000)
	c.rebuild()

	prng := rand.New(rand.NewPCG(20, 20))
	addresses := make([]netip.Addr, 1024)
	for i := range addresses {
		addresses[i] = helpers.AddrTo6(benchmarkAddr(prng))
	}

	i := 0
	for b.Loop() {
		c.Lookup(addresses[i%len(addresses)])
		i++
	}
}
