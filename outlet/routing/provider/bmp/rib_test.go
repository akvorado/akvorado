// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"fmt"
	"math/rand"
	"net/netip"
	"testing"
	"unsafe"

	"akvorado/common/helpers"

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
		t.Errorf("Alignment error for large community slices. Got %d, expected 12",
			diff)
	}

	// Also check other stuff we think are true about "unsafe"
	if unsafe.Sizeof(largeCommunities[0]) != 12 {
		t.Errorf("Large community size: got %d, expected 12", unsafe.Sizeof(largeCommunities[0]))
	}
	const _ = unsafe.Sizeof(largeCommunities[0])
}

func TestRTAEqual(t *testing.T) {
	cases := []struct {
		pos   helpers.Pos
		rta1  routeAttributes
		rta2  routeAttributes
		equal bool
	}{
		{helpers.Mark(), routeAttributes{asn: 2038}, routeAttributes{asn: 2038}, true},
		{helpers.Mark(), routeAttributes{asn: 2038}, routeAttributes{asn: 2039}, false},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, asPath: []uint32{}},
			routeAttributes{asn: 2038},
			true,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, asPath: []uint32{}},
			routeAttributes{asn: 2039},
			false,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, communities: []uint32{}},
			routeAttributes{asn: 2038},
			true,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, communities: []uint32{}},
			routeAttributes{asn: 2039},
			false,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{}},
			routeAttributes{asn: 2038},
			true,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{}},
			routeAttributes{asn: 2039},
			false,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3}},
			true,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4}},
			false,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 0}},
			false,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 4}},
			false,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4}},
			true,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34}},
			true,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34}},
			routeAttributes{asn: 2038, asPath: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 35}},
			false,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, communities: []uint32{100, 200, 300, 400}},
			routeAttributes{asn: 2038, communities: []uint32{100, 200, 300, 400}},
			true,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, communities: []uint32{100, 200, 300, 400}},
			routeAttributes{asn: 2038, communities: []uint32{100, 200, 300, 402}},
			false,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, communities: []uint32{100, 200, 300}},
			routeAttributes{asn: 2038, communities: []uint32{100, 200, 300, 400}},
			false,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 7}}},
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 7}}},
			true,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 7}}},
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 8}}},
			false,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 7}}},
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 4}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 7}}},
			false,
		},
		{
			helpers.Mark(),
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}}},
			routeAttributes{asn: 2038, largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}, {ASN: 3, LocalData1: 4, LocalData2: 5}, {ASN: 5, LocalData1: 6, LocalData2: 7}}},
			false,
		},
	}

	for _, tc := range cases {
		equal := tc.rta1.Equal(tc.rta2)
		if equal && !tc.equal {
			t.Errorf("%s%+v == %+v", tc.pos, tc.rta1, tc.rta2)
		} else if !equal && tc.equal {
			t.Errorf("%s%+v != %+v", tc.pos, tc.rta1, tc.rta2)
		} else {
			equal := tc.rta1.Hash() == tc.rta2.Hash()
			if equal && !tc.equal {
				t.Errorf("%s%+v.hash == %+v.hash", tc.pos, tc.rta1, tc.rta2)
			} else if !equal && tc.equal {
				t.Errorf("%s%+v.hash != %+v.hash", tc.pos, tc.rta1, tc.rta2)
			}
		}
	}
}

func TestRemoveRoutes(t *testing.T) {
	nr := func(r *rib, peer uint32) route {
		return route{
			peer:    peer,
			nlri:    r.nlris.Put(nlri{family: bgp.RF_IPv4_UC, path: 1}),
			nextHop: r.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:198.51.100.8"))),
			attributes: r.rtas.Put(routeAttributes{
				asn: 65300,
			}),
			prefixLen: 96 + 24,
		}
	}
	t.Run("only route", func(t *testing.T) {
		r := newRIB()
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), nr(r, 10))
		idx, _ := r.tree.Lookup(netip.MustParseAddr("::ffff:192.168.144.10"))
		count, empty := r.removeRoutes(idx, func(route) bool { return true }, true)
		if !empty {
			t.Error("removeRoutes() should have removed all routes from node")
		}
		if count != 1 {
			t.Error("removeRoutes() should have removed 1 route")
		}
		if diff := helpers.Diff(r.routes, map[prefixIndex]route{}); diff != "" {
			t.Errorf("removeRoutes() (-got, +want):\n%s", diff)
		}
	})

	t.Run("first route", func(t *testing.T) {
		r := newRIB()
		r1 := nr(r, 10)
		r2 := nr(r, 11)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r1)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r2)
		idx, _ := r.tree.Lookup(netip.MustParseAddr("::ffff:192.168.144.10"))
		count, empty := r.removeRoutes(idx, func(r route) bool { return r.peer == 10 }, true)
		if empty {
			t.Error("removeRoutes() should not have removed all routes from node")
		}
		if count != 1 {
			t.Error("removeRoutes() should have removed 1 route")
		}
		if diff := helpers.Diff(r.routes, map[routeKey]route{
			makeRouteKey(idx, 0): r2,
		}); diff != "" {
			t.Errorf("removeRoutes() (-got, +want):\n%s", diff)
		}
	})

	t.Run("second route", func(t *testing.T) {
		r := newRIB()
		r1 := nr(r, 10)
		r2 := nr(r, 11)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r1)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r2)
		idx, _ := r.tree.Lookup(netip.MustParseAddr("::ffff:192.168.144.10"))
		count, empty := r.removeRoutes(idx, func(r route) bool { return r.peer == 11 }, true)
		if empty {
			t.Error("removeRoutes() should not have removed all routes from node")
		}
		if count != 1 {
			t.Error("removeRoutes() should have removed 1 route")
		}
		if diff := helpers.Diff(r.routes, map[routeKey]route{
			makeRouteKey(idx, 0): r1,
		}); diff != "" {
			t.Errorf("removeRoutes() (-got, +want):\n%s", diff)
		}
	})
	t.Run("middle route", func(t *testing.T) {
		r := newRIB()
		r1 := nr(r, 10)
		r2 := nr(r, 11)
		r3 := nr(r, 12)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r1)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r2)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r3)
		idx, _ := r.tree.Lookup(netip.MustParseAddr("::ffff:192.168.144.10"))
		count, empty := r.removeRoutes(idx, func(r route) bool { return r.peer == 11 }, true)
		if empty {
			t.Error("removeRoutes() should not have removed all routes from node")
		}
		if count != 1 {
			t.Error("removeRoutes() should have removed 1 route")
		}
		if diff := helpers.Diff(r.routes, map[routeKey]route{
			makeRouteKey(idx, 0): r1,
			makeRouteKey(idx, 1): r3,
		}); diff != "" {
			t.Errorf("removeRoutes() (-got, +want):\n%s", diff)
		}
	})
	t.Run("one route out of two", func(t *testing.T) {
		r := newRIB()
		r1 := nr(r, 10)
		r2 := nr(r, 11)
		r3 := nr(r, 12)
		r4 := nr(r, 13)
		r5 := nr(r, 14)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r1)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r2)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r3)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r4)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r5)
		idx, _ := r.tree.Lookup(netip.MustParseAddr("::ffff:192.168.144.10"))
		count, empty := r.removeRoutes(idx, func(r route) bool { return r.peer%2 == 0 }, false)
		if empty {
			t.Error("removeRoutes() should not have removed all routes from node")
		}
		if count != 3 {
			t.Error("removeRoutes() should have removed 3 route")
		}
		if diff := helpers.Diff(r.routes, map[routeKey]route{
			makeRouteKey(idx, 0): r2,
			makeRouteKey(idx, 1): r4,
		}); diff != "" {
			t.Errorf("removeRoutes() (-got, +want):\n%s", diff)
		}
	})

	t.Run("all routes", func(t *testing.T) {
		r := newRIB()
		r1 := nr(r, 10)
		r2 := nr(r, 11)
		r3 := nr(r, 12)
		r4 := nr(r, 13)
		r5 := nr(r, 14)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r1)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r2)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r3)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r4)
		r.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r5)
		idx, _ := r.tree.Lookup(netip.MustParseAddr("::ffff:192.168.144.10"))
		count, empty := r.removeRoutes(idx, func(route) bool { return true }, false)
		if !empty {
			t.Error("removeRoutes() should have removed all routes from node")
		}
		if count != 5 {
			t.Error("removeRoutes() should have removed 5 route")
		}
		if diff := helpers.Diff(r.routes, map[routeKey]route{}); diff != "" {
			t.Errorf("removeRoutes() (-got, +want):\n%s", diff)
		}
	})
}

func TestRIBHarness(t *testing.T) {
	for run := range 5 {
		random := rand.New(rand.NewSource(int64(run)))
		run++

		// Ramp up the test
		totalExporters := run
		peerPerExporter := 1 + run/2
		maxInitialRoutePerPeer := 500 * run
		maxRemovedRoutePerPeer := 100 * run
		maxReaddedRoutePerPeer := 50 * run
		t.Logf("Run %d. Exporters=%d, Peers=%d, Initial=max %d, Removed=max %d, Readded=max %d",
			run, totalExporters, peerPerExporter,
			maxInitialRoutePerPeer, maxRemovedRoutePerPeer, maxReaddedRoutePerPeer)

		r := newRIB()
		type lookup struct {
			peer    uint32
			addr    netip.Addr
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
				if lookups[idx].addr != lookup.addr || lookups[idx].rd != lookup.rd {
					continue
				}
				if lookups[idx].removed {
					continue
				}
				lookups[idx].removed = true
				break
			}
		}

		peers := []uint32{}
		for i := range totalExporters {
			for j := range peerPerExporter {
				peer := uint32((i << 16) + int(j))
				peers = append(peers, peer)
				toAdd := random.Intn(maxInitialRoutePerPeer)
				added := 0
				for range toAdd {
					lookup := lookup{
						peer: peer,
						addr: netip.MustParseAddr(fmt.Sprintf("2001:db8:f:%x::",
							random.Intn(300))),
						nextHop: netip.MustParseAddr(
							fmt.Sprintf("2001:db8:c::%x", random.Intn(500))),
						rd:  RD(random.Intn(3)),
						asn: uint32(random.Intn(1000)),
					}
					added += r.addPrefix(netip.PrefixFrom(lookup.addr, 64),
						route{
							peer:    peer,
							nlri:    r.nlris.Put(nlri{rd: lookup.rd}),
							nextHop: r.nextHops.Put(nextHop(lookup.nextHop)),
							attributes: r.rtas.Put(routeAttributes{
								asn: lookup.asn,
							}),
						})
					removeLookup(lookup)
					lookups = append(lookups, lookup)
				}
				t.Logf("Run %d: added = %d/%d", run, added, toAdd)

				toRemove := random.Intn(maxRemovedRoutePerPeer)
				removed := 0
				for range toRemove {
					prefix := netip.MustParseAddr(fmt.Sprintf("2001:db8:f:%x::",
						random.Intn(300)))
					rd := RD(random.Intn(4))
					if nlriRef, ok := r.nlris.Ref(nlri{
						rd: rd,
					}); ok {
						removed += r.removePrefix(netip.PrefixFrom(prefix, 64),
							route{
								peer: peer,
								nlri: nlriRef,
							})
						removeLookup(lookup{
							peer: peer,
							addr: prefix,
							rd:   rd,
						})
					}
				}
				t.Logf("Run %d: removed = %d/%d", run, removed, toRemove)

				toAdd = random.Intn(maxReaddedRoutePerPeer)
				added = 0
				for range toAdd {
					lookup := lookup{
						peer: peer,
						addr: netip.MustParseAddr(fmt.Sprintf("2001:db8:f:%x::",
							random.Intn(300))),
						nextHop: netip.MustParseAddr(
							fmt.Sprintf("2001:db8:c::%x", random.Uint32()%500)),
						asn: uint32(random.Intn(1010)),
					}
					added += r.addPrefix(netip.PrefixFrom(lookup.addr, 64),
						route{
							peer:    peer,
							nlri:    r.nlris.Put(nlri{}),
							nextHop: r.nextHops.Put(nextHop(lookup.nextHop)),
							attributes: r.rtas.Put(routeAttributes{
								asn: lookup.asn,
							}),
						})
					removeLookup(lookup)
					lookups = append(lookups, lookup)
				}
				t.Logf("Run %d: readedd = %d/%d", run, added, toAdd)
			}
		}

		removed := 0
		for _, lookup := range lookups {
			if lookup.removed {
				removed++
				continue
			}
			// Find prefix in tree
			prefixIdx, ok := r.tree.Lookup(lookup.addr)
			if !ok {
				t.Errorf("cannot find %s for %d",
					lookup.addr, lookup.peer)
				continue
			}

			// Check if routes exist for this prefix
			found := false
			routeFound := false

			for route := range r.iterateRoutesForPrefixIndex(prefixIdx) {
				routeFound = true // At least one route exists
				if r.nextHops.Get(route.nextHop) != nextHop(lookup.nextHop) || r.nlris.Get(route.nlri).rd != lookup.rd {
					continue
				}
				if r.rtas.Get(route.attributes).asn != lookup.asn {
					continue
				}
				found = true
				break
			}

			if !routeFound {
				t.Errorf("no routes found for %s for %d",
					lookup.addr, lookup.peer)
				continue
			}

			if !found {
				t.Logf("Available routes for %s:", lookup.addr)
				for route := range r.iterateRoutesForPrefixIndex(prefixIdx) {
					t.Logf("peer %d, NH: %s, RD: %s, ASN: %d",
						route.peer,
						netip.Addr(r.nextHops.Get(route.nextHop)),
						r.nlris.Get(route.nlri).rd, r.rtas.Get(route.attributes).asn)
				}
				t.Errorf("cannot find %s for peer %d; NH: %s, RD: %s, ASN: %d",
					lookup.addr, lookup.peer,
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

		// Check for leak of interned values
		if r.nlris.Len() > 0 {
			t.Errorf("%d NLRIs have leaked", r.nlris.Len())
		}
		if r.nextHops.Len() > 0 {
			t.Errorf("%d next hops have leaked", r.nextHops.Len())
		}
		if r.rtas.Len() > 0 {
			t.Errorf("%d route attributes have leaked", r.rtas.Len())
		}

		if t.Failed() {
			break
		}
	}
}

func BenchmarkRTAHash(b *testing.B) {
	rta := routeAttributes{
		asn:    2038,
		asPath: []uint32{1, 2, 3, 4, 5, 6, 7},
	}
	for b.Loop() {
		rta.Hash()
	}
}

func BenchmarkRTAEqual(b *testing.B) {
	rta := routeAttributes{
		asn:    2038,
		asPath: []uint32{1, 2, 3, 4, 5, 6, 7},
	}
	for b.Loop() {
		rta.Equal(rta)
	}
}
