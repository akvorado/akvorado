// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"fmt"
	"iter"
	"math/rand"
	"net/netip"
	"runtime"
	"slices"
	"testing"

	"akvorado/common/helpers"

	"github.com/kentik/patricia"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
)

var asPathCache [][]uint32
var uniqueASPaths = 254123

// Data from https://bgp.potaroo.net/as2.0/bgp-prefix-vector.txt
var prefixSizeDistribution = [33]int{
	0, // /0
	0, 0, 0, 0, 0, 0, 0, 16,
	14, 41, 92, 298, 581, 1210, 2161, 13854,
	8369, 13785, 25080, 45910, 53311, 114177, 108013, 632625,
	793, 0, 0, 0, 0, 0, 0, 0,
}

func init() {
	prng := rand.New(rand.NewSource(0))
	// Data from https://bgp.potaroo.net/as2.0/bgp-asbyhop-vector.txt
	asDistanceDistribution := []int{
		1, 2, 366, 9645, 29251, 28984, 7340, 1423, 383, 63, 14, 2, 4, 4, 4, 4, 1, 0,
	}

	totalAS := 0
	for _, v := range asDistanceDistribution {
		totalAS += v
	}

	// Generate a cache for the AS paths. We should pick one at random.
	asPathCache = make([][]uint32, uniqueASPaths)
	for i := range uniqueASPaths {
		// Generate AS path length based on distribution
		asPathLen := 1
		r := prng.Intn(totalAS)
		cumulative := 0
		for len, count := range asDistanceDistribution {
			cumulative += count
			if r < cumulative {
				asPathLen = len
				if asPathLen == 0 {
					asPathLen = 1
				}
				break
			}
		}

		// Generate unique AS path
		asPath := make([]uint32, asPathLen)
		for j := 0; j < asPathLen; j++ {
			asPath[j] = uint32(prng.Intn(64494) + 1)
		}
		asPathCache[i] = asPath
	}
}

type randomRoute struct {
	Prefix           netip.Prefix
	ASPath           []uint32
	Communities      []uint32
	LargeCommunities []bgp.LargeCommunity
}

// randomRealWorldRoutes4 generates random IPv4 routes matching the size distribution
// found on the Internet. Using data from https://bgp.potaroo.net/as2.0/index.html
func randomRealWorldRoutes4(prngPrefixes, prngASPaths *rand.Rand, n int) iter.Seq[randomRoute] {
	totalRoutes := 0
	for _, v := range prefixSizeDistribution {
		totalRoutes += v
	}

	return func(yield func(randomRoute) bool) {
		for range n {
			// Generate prefix length based on distribution
			prefixLen := 0
			r := prngPrefixes.Intn(totalRoutes)
			cumulative := 0
			for len, count := range prefixSizeDistribution {
				cumulative += count
				if r < cumulative {
					prefixLen = len
					break
				}
			}

			// Generate random IPv4 prefix
			ip := netip.AddrFrom4([4]byte{
				byte(prngPrefixes.Intn(224)),
				byte(prngPrefixes.Intn(256)),
				byte(prngPrefixes.Intn(256)),
				byte(prngPrefixes.Intn(256)),
			})
			prefix := netip.PrefixFrom(ip, prefixLen).Masked()

			// Select a random AS path from the pre-generated cache
			asPath := asPathCache[prngASPaths.Intn(len(asPathCache))]

			// Generate communities (0-5 communities per route)
			numCommunities := max(0, prngASPaths.Intn(10)-4)
			communities := make([]uint32, numCommunities)
			for j := range numCommunities {
				asFromPath := asPath[prngASPaths.Intn(len(asPath))]
				communities[j] = asFromPath<<16 | uint32(prngASPaths.Intn(3))
			}
			slices.Sort(communities)
			communities = slices.Compact(communities)

			// Generate large communities (0-3 per route, but they are rare)
			numLargeCommunities := max(0, prngASPaths.Intn(100)-97)
			largeCommunities := make([]bgp.LargeCommunity, numLargeCommunities)
			for j := range numLargeCommunities {
				largeCommunities[j] = bgp.LargeCommunity{
					ASN:        asPath[prngASPaths.Intn(len(asPath))],
					LocalData1: uint32(prngASPaths.Intn(2)) + 1,
					LocalData2: uint32(prngASPaths.Intn(2)) + 1,
				}
			}

			route := randomRoute{
				Prefix:           prefix,
				ASPath:           asPath,
				Communities:      communities,
				LargeCommunities: largeCommunities,
			}

			if !yield(route) {
				return
			}
		}
	}
}

func TestRandomRealWorldRoutes4(t *testing.T) {
	prng1 := rand.New(rand.NewSource(1))
	prng2 := rand.New(rand.NewSource(2))
	routes := []randomRoute{}
	for route := range randomRealWorldRoutes4(prng1, prng2, 2) {
		routes = append(routes, route)
	}
	expectedRoutes := []randomRoute{
		{
			Prefix:           netip.MustParsePrefix("79.199.187.0/24"),
			ASPath:           []uint32{29418, 57855, 38297},
			Communities:      []uint32{1927938050, 2509832194},
			LargeCommunities: []bgp.LargeCommunity{},
		},
		{
			Prefix:           netip.MustParsePrefix("185.172.72.0/24"),
			ASPath:           []uint32{25258, 9490, 64459, 11892, 37685},
			Communities:      []uint32{},
			LargeCommunities: []bgp.LargeCommunity{},
		},
	}
	if diff := helpers.Diff(routes, expectedRoutes); diff != "" {
		t.Fatalf("randomRealWorldRoutes4() (-got, +want):\n%s", diff)
	}
}

func TestRandomRealWorldRoutes4Distribution(t *testing.T) {
	prng1 := rand.New(rand.NewSource(42))
	prng2 := rand.New(rand.NewSource(43))

	totalRoutes := 0
	for _, v := range prefixSizeDistribution {
		totalRoutes += v
	}

	// Generate many routes and count prefix lengths
	n := 1_000_000
	prefixLengthCounts := make(map[int]int)
	for route := range randomRealWorldRoutes4(prng1, prng2, n) {
		prefixLen := route.Prefix.Bits()
		prefixLengthCounts[prefixLen]++
	}

	// Check distribution within 10% margin
	for prefixLen := range 33 {
		expected := float64(prefixSizeDistribution[prefixLen]) / float64(totalRoutes) * float64(n)
		actual := float64(prefixLengthCounts[prefixLen])

		if expected > 0 {
			errorMargin := 0.3 * expected
			if actual < expected-errorMargin || actual > expected+errorMargin {
				t.Errorf("Prefix length /%d: expected %.1f±%.1f, got %.0f",
					prefixLen, expected, errorMargin, actual)
			}
		} else if actual > 0 {
			t.Errorf("Prefix length /%d: expected 0, got %d", prefixLen, prefixLengthCounts[prefixLen])
		}
	}
}

func BenchmarkRandomRealWorldRoutes4(b *testing.B) {
	prng1 := rand.New(rand.NewSource(1))
	prng2 := rand.New(rand.NewSource(2))
	for b.Loop() {
		for route := range randomRealWorldRoutes4(prng1, prng2, 1000) {
			_ = route
		}
	}
	b.ReportMetric(0, "ns/op")
	b.ReportMetric(float64(b.Elapsed())/float64(b.N)/1000, "ns/route")
}

func BenchmarkRIBInsertion(b *testing.B) {
	for _, routes := range []int{1_000, 10_000, 100_000} {
		for _, peers := range []int{1, 2, 5} {
			name := fmt.Sprintf("%d routes, %d peers", routes, peers)

			b.Run(name, func(b *testing.B) {
				var startMem, endMem runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&startMem)

				var rib *rib
				inserted := 0
				tentative := 0
				for b.Loop() {
					rib = newRIB()
					nh := netip.MustParseAddr("::ffff:198.51.100.0")
					prng1 := rand.New(rand.NewSource(10))
					prng2 := make([]*rand.Rand, peers)
					for p := range peers {
						prng2[p] = rand.New(rand.NewSource(int64(p)))
					}
					for p := range peers {
						nh = nh.Next()
						for r := range randomRealWorldRoutes4(prng1, prng2[p], routes) {
							if prng2[p].Intn(10) == 0 {
								continue
							}
							pfx := netip.PrefixFrom(netip.AddrFrom16(r.Prefix.Addr().As16()), r.Prefix.Bits()+96)
							tentative++
							inserted += rib.addPrefix(pfx.Addr(), pfx.Bits(), route{
								peer:    uint32(p),
								nlri:    rib.nlris.Put(nlri{family: bgp.RF_IPv4_UC}),
								nextHop: rib.nextHops.Put(nextHop(nh)),
								attributes: rib.rtas.Put(routeAttributes{
									asn:              r.ASPath[len(r.ASPath)-1],
									asPath:           r.ASPath,
									communities:      r.Communities,
									largeCommunities: r.LargeCommunities,
									plen:             uint8(pfx.Bits()),
								}),
							})
						}
					}
				}
				runtime.GC()
				runtime.ReadMemStats(&endMem)
				b.ReportMetric(0, "ns/op")
				b.ReportMetric(float64(b.Elapsed())/float64(inserted), "ns/route")
				b.ReportMetric(float64(endMem.HeapAlloc-startMem.HeapAlloc)/float64(rib.tree.CountTags()), "bytes/route")
				b.ReportMetric(float64(inserted)/float64(tentative)*100, "%ins")

				// Avoid elimination of the RIB
				rib.tree.FindDeepestTags(patricia.NewIPv6Address(netip.MustParseAddr("::ffff:192.168.1.1").AsSlice(), 128))
			})
		}
	}
}

func BenchmarkRIBLookup(b *testing.B) {
	for _, routes := range []int{1_000, 10_000, 100_000} {
		for _, peers := range []int{1, 2, 5} {
			name := fmt.Sprintf("%d routes, %d peers", routes, peers)

			b.Run(name, func(b *testing.B) {
				rib := newRIB()
				nh := netip.MustParseAddr("::ffff:198.51.100.0")
				prng1 := rand.New(rand.NewSource(10))
				prng2 := make([]*rand.Rand, peers)
				for p := range peers {
					prng2[p] = rand.New(rand.NewSource(int64(p)))
				}
				for p := range peers {
					nh = nh.Next()
					for r := range randomRealWorldRoutes4(prng1, prng2[p], routes) {
						if prng2[p].Intn(10) == 0 {
							continue
						}
						pfx := netip.PrefixFrom(netip.AddrFrom16(r.Prefix.Addr().As16()), r.Prefix.Bits()+96)
						rib.addPrefix(pfx.Addr(), pfx.Bits(), route{
							peer:    uint32(p),
							nlri:    rib.nlris.Put(nlri{family: bgp.RF_IPv4_UC}),
							nextHop: rib.nextHops.Put(nextHop(nh)),
							attributes: rib.rtas.Put(routeAttributes{
								asn:              r.ASPath[len(r.ASPath)-1],
								asPath:           r.ASPath,
								communities:      r.Communities,
								largeCommunities: r.LargeCommunities,
								plen:             uint8(pfx.Bits()),
							}),
						})
					}
				}

				prng1 = rand.New(rand.NewSource(10))
				lookups := 0
				for b.Loop() {
					for r := range randomRealWorldRoutes4(prng1, prng2[0], routes/10) {
						addr := r.Prefix.Addr().As16()
						_, _ = rib.tree.FindDeepestTags(patricia.NewIPv6Address(addr[:], 128))
						lookups++
					}
				}
				b.ReportMetric(float64(b.Elapsed())/float64(lookups), "ns/op")
			})
		}
	}
}

func BenchmarkRIBFlush(b *testing.B) {
	for _, routes := range []int{1_000, 10_000, 100_000} {
		for _, peers := range []int{1, 2, 5} {
			name := fmt.Sprintf("%d routes, %d peers", routes, peers)

			b.Run(name, func(b *testing.B) {
				for b.Loop() {
					b.StopTimer()
					rib := newRIB()
					nh := netip.MustParseAddr("::ffff:198.51.100.0")
					prng1 := rand.New(rand.NewSource(10))
					prng2 := make([]*rand.Rand, peers)
					for p := range peers {
						prng2[p] = rand.New(rand.NewSource(int64(p)))
					}
					for p := range peers {
						nh = nh.Next()
						for r := range randomRealWorldRoutes4(prng1, prng2[p], routes) {
							if prng2[p].Intn(10) == 0 {
								continue
							}
							pfx := netip.PrefixFrom(netip.AddrFrom16(r.Prefix.Addr().As16()), r.Prefix.Bits()+96)
							rib.addPrefix(pfx.Addr(), pfx.Bits(), route{
								peer:    uint32(p),
								nlri:    rib.nlris.Put(nlri{family: bgp.RF_IPv4_UC}),
								nextHop: rib.nextHops.Put(nextHop(nh)),
								attributes: rib.rtas.Put(routeAttributes{
									asn:              r.ASPath[len(r.ASPath)-1],
									asPath:           r.ASPath,
									communities:      r.Communities,
									largeCommunities: r.LargeCommunities,
									plen:             uint8(pfx.Bits()),
								}),
							})
						}
					}

					b.StartTimer()
					rib.flushPeer(0)
				}
				b.ReportMetric(0, "ns/op")
				b.ReportMetric(float64(b.Elapsed())/float64(b.N)/1_000_000, "ms/op")
			})
		}
	}
}
