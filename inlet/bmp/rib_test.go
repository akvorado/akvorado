// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"fmt"
	"math/rand"
	"net/netip"
	"testing"
	"unsafe"

	"github.com/kentik/patricia"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
)

func TestLargeCommunitiesAlign(t *testing.T) {
	largeCommunities := []bgp.LargeCommunity{
		{ASN: 1, LocalData1: 2, LocalData2: 3},
		{ASN: 4, LocalData1: 5, LocalData2: 6},
	}
	first := unsafe.Pointer(&largeCommunities[0])
	second := unsafe.Pointer(&largeCommunities[1])
	diff := uintptr(second) - uintptr(first)
	if diff != 12 {
		t.Fatalf("Alignment error for large community slices. Got %d, expected 12",
			diff)
	}
}

func TestRTAEqual(t *testing.T) {
	cases := []struct {
		rta1  routeAttributes
		rta2  routeAttributes
		equal bool
	}{
		{routeAttributes{asn: 2038}, routeAttributes{asn: 2038}, true},
		{routeAttributes{asn: 2038}, routeAttributes{asn: 2039}, false},
		{
			routeAttributes{asn: 2038, asPath: []uint32{}},
			routeAttributes{asn: 2038},
			true,
		}, {
			routeAttributes{asn: 2038, asPath: []uint32{}},
			routeAttributes{asn: 2039},
			false,
		}, {
			routeAttributes{asn: 2038, communities: []uint32{}},
			routeAttributes{asn: 2038},
			true,
		}, {
			routeAttributes{asn: 2038, communities: []uint32{}},
			routeAttributes{asn: 2039},
			false,
		}, {
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{}},
			routeAttributes{asn: 2038},
			true,
		}, {
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{}},
			routeAttributes{asn: 2039},
			false,
		}, {
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3}},
			true,
		}, {
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4}},
			false,
		}, {
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 0}},
			false,
		}, {
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 4}},
			false,
		}, {
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4}},
			true,
		}, {
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34}},
			true,
		}, {
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 35}},
			false,
		}, {
			routeAttributes{asn: 2038, communities: []uint32{100, 200, 300, 400}},
			routeAttributes{asn: 2038, communities: []uint32{100, 200, 300, 400}},
			true,
		}, {
			routeAttributes{asn: 2038, communities: []uint32{100, 200, 300, 400}},
			routeAttributes{asn: 2038, communities: []uint32{100, 200, 300, 402}},
			false,
		}, {
			routeAttributes{asn: 2038, communities: []uint32{100, 200, 300}},
			routeAttributes{asn: 2038, communities: []uint32{100, 200, 300, 400}},
			false,
		}, {
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 7}}},
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 7}}},
			true,
		}, {
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 7}}},
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 8}}},
			false,
		}, {
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 7}}},
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 4}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 7}}},
			false,
		}, {
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}}},
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 7}}},
			false,
		},
	}
outer:
	for try := 3; try >= 0; try-- {
		// We may have to try a few times because of
		// collisions due to reduced hash efficiency during
		// tests.
		for _, tc := range cases {
			equal := tc.rta1.Equal(tc.rta2)
			if equal && !tc.equal {
				t.Errorf("%+v == %+v", tc.rta1, tc.rta2)
			} else if !equal && tc.equal {
				t.Errorf("%+v != %+v", tc.rta1, tc.rta2)
			} else {
				equal := tc.rta1.Hash() == tc.rta2.Hash()
				if equal && !tc.equal {
					if try > 0 {
						// We may have a collision, change the seed and retry
						rtaHashSeed = rand.Uint64()
						continue outer
					}
					t.Errorf("%+v.hash == %+v.hash", tc.rta1, tc.rta2)
				} else if !equal && tc.equal {
					t.Errorf("%+v.hash != %+v.hash", tc.rta1, tc.rta2)
				}
			}
		}
	}
}

func TestRIB(t *testing.T) {
	for i := 0; i < 5; i++ {
		t.Logf("Run %d", i+1)
		r := newRIB()
		random := rand.New(rand.NewSource(100 * int64(i)))
		type lookup struct {
			peer    uint32
			prefix  netip.Addr // Assume /64
			nextHop netip.Addr
			rd      RD
			asn     uint32
			removed bool
		}
		// We store all lookups that should succeed
		lookups := []lookup{}
		removeLookup := func(lookup lookup) {
			for idx := range lookups {
				if lookups[idx].peer != lookup.peer {
					continue
				}
				if lookups[idx].prefix != lookup.prefix || lookups[idx].rd != lookup.rd {
					continue
				}
				if lookups[idx].removed {
					continue
				}
				lookups[idx].removed = true
				break
			}
		}

		totalExporters := 20
		peers := []uint32{}
		for i := 0; i < totalExporters; i++ {
			for j := 0; j < int(random.Uint32()%14); j++ {
				peer := uint32((i << 16) + j)
				peers = append(peers, peer)
				for k := 0; k < int(random.Uint32()%10000); k++ {
					lookup := lookup{
						peer: peer,
						prefix: netip.MustParseAddr(fmt.Sprintf("2001:db8:f:%x::",
							random.Uint32()%300)),
						nextHop: netip.MustParseAddr(
							fmt.Sprintf("2001:db8:c::%x", random.Uint32()%500)),
						rd:  RD(random.Uint64() % 3),
						asn: random.Uint32() % 1000,
					}
					r.addPrefix(lookup.prefix, 64,
						route{
							peer:    peer,
							nlri:    nlri{rd: lookup.rd},
							nextHop: r.nextHops.Put(nextHop(lookup.nextHop)),
							attributes: r.rtas.Put(routeAttributes{
								asn: lookup.asn,
							}),
						})
					removeLookup(lookup)
					lookups = append(lookups, lookup)
				}
				for k := 0; k < int(random.Uint32()%500); k++ {
					prefix := netip.MustParseAddr(fmt.Sprintf("2001:db8:f:%x::",
						random.Uint32()%300))
					rd := RD(random.Uint64() % 4)
					r.removePrefix(prefix, 64,
						route{
							peer: peer,
							nlri: nlri{
								rd: rd,
							},
						})
					removeLookup(lookup{
						peer:   peer,
						prefix: prefix,
						rd:     rd,
					})
				}
				for k := 0; k < int(random.Uint32()%200); k++ {
					lookup := lookup{
						peer: peer,
						prefix: netip.MustParseAddr(fmt.Sprintf("2001:db8:f:%x::",
							random.Uint32()%300)),
						nextHop: netip.MustParseAddr(
							fmt.Sprintf("2001:db8:c::%x", random.Uint32()%500)),
						asn: random.Uint32() % 1010,
					}
					r.addPrefix(lookup.prefix, 64,
						route{
							peer:    peer,
							nextHop: r.nextHops.Put(nextHop(lookup.nextHop)),
							attributes: r.rtas.Put(routeAttributes{
								asn: lookup.asn,
							}),
						})
					removeLookup(lookup)
					lookups = append(lookups, lookup)
				}
			}
		}

		removed := 0
		for _, lookup := range lookups {
			if lookup.removed {
				removed++
				continue
			}
			v6 := patricia.NewIPv6Address(lookup.prefix.AsSlice(), 128)
			ok, tags := r.tree.FindDeepestTags(v6)
			if !ok {
				t.Errorf("cannot find %s/128 for %d",
					lookup.prefix, lookup.peer)
			}
			found := false
			for _, route := range tags {
				if r.nextHops.Get(route.nextHop) != nextHop(lookup.nextHop) || route.nlri.rd != lookup.rd {
					continue
				}
				if r.rtas.Get(route.attributes).asn != lookup.asn {
					continue
				}
				found = true
				break
			}
			if !found {
				for _, route := range tags {
					t.Logf("route NH: %s, RD: %s, ASN: %d",
						netip.Addr(r.nextHops.Get(route.nextHop)),
						route.nlri.rd, r.rtas.Get(route.attributes).asn)
				}
				t.Errorf("cannot find %s/128 for %d; NH: %s, RD: %s, ASN: %d",
					lookup.prefix, lookup.peer,
					lookup.nextHop, lookup.rd, lookup.asn)
			}
		}
		if removed < 5 {
			t.Error("did not remove more than 5 prefixes, suspicious...")
		}

		// Remove everything
		for _, peer := range peers {
			r.flushPeer(peer)
		}

		// Check for leak of route attributes
		if r.rtas.Len() > 0 {
			t.Fatalf("%d route attributes have leaked", r.rtas.Len())
		}
	}
}

func BenchmarkRTAHash(b *testing.B) {
	rta := routeAttributes{
		asn:    2038,
		asPath: []uint32{1, 2, 3, 4, 5, 6, 7},
	}
	for n := 0; n < b.N; n++ {
		rta.Hash()
	}
}

func BenchmarkRTAEqual(b *testing.B) {
	rta := routeAttributes{
		asn:    2038,
		asPath: []uint32{1, 2, 3, 4, 5, 6, 7},
	}
	for n := 0; n < b.N; n++ {
		rta.Equal(rta)
	}
}
