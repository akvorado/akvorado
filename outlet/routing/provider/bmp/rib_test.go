// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"fmt"
	"math/rand/v2"
	"net/netip"
	"testing"
	"unique"
	"unsafe"

	"akvorado/common/helpers"

	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"github.com/puzpuzpuz/xsync/v4"
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
	// Compile-time assertion: LargeCommunity must be exactly 12 bytes
	const _ = unsafe.Sizeof(bgp.LargeCommunity{}) - 12
}

func TestRouteAttributesEncoding(t *testing.T) {
	cases := []struct {
		name string
		rta  routeAttributes
	}{
		{"all empty", routeAttributes{asn: 100}},
		{"only asPath", routeAttributes{asn: 200, asPath: []uint32{1, 2, 3}}},
		{"only communities", routeAttributes{asn: 300, communities: []uint32{100, 200}}},
		{"only largeCommunities", routeAttributes{
			asn:              400,
			largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}},
		}},
		{"all fields", routeAttributes{
			asn:         500,
			asPath:      []uint32{1, 2, 3, 100, 200, 65535, 4294967295},
			communities: []uint32{42},
			largeCommunities: []bgp.LargeCommunity{
				{ASN: 64200, LocalData1: 100, LocalData2: 200},
				{ASN: 65017, LocalData1: 300, LocalData2: 400},
				{ASN: 4294967295, LocalData1: 4294967295, LocalData2: 4294967295},
			},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			comparable := tc.rta.ToComparable()
			if comparable.asn != tc.rta.asn {
				t.Errorf("asn: got %d, want %d", comparable.asn, tc.rta.asn)
			}
			if diff := helpers.Diff(comparable.getASPath(), tc.rta.asPath); diff != "" {
				t.Errorf("asPath roundtrip (-got, +want):\n%s", diff)
			}
			if diff := helpers.Diff(comparable.getCommunities(), tc.rta.communities); diff != "" {
				t.Errorf("communities roundtrip (-got, +want):\n%s", diff)
			}
			if diff := helpers.Diff(comparable.getLargeCommunities(), tc.rta.largeCommunities); diff != "" {
				t.Errorf("largeCommunities roundtrip (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestRouteAttributesComparable(t *testing.T) {
	rta1 := routeAttributes{
		asn: 2038, asPath: []uint32{1, 2, 3}, communities: []uint32{100, 200},
		largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}},
	}.ToComparable()
	rta2 := routeAttributes{
		asn: 2038, asPath: []uint32{1, 2, 3}, communities: []uint32{100, 200},
		largeCommunities: []bgp.LargeCommunity{{ASN: 1, LocalData1: 2, LocalData2: 3}},
	}.ToComparable()
	rta3 := routeAttributes{
		asn: 2038, asPath: []uint32{1, 2, 4}, communities: []uint32{100, 200},
	}.ToComparable()

	if rta1 != rta2 {
		t.Error("identical routeAttributesComparable should be ==")
	}
	if rta1 == rta3 {
		t.Error("different routeAttributesComparable should be !=")
	}

	// Also verify unique.Make deduplicates
	h1 := unique.Make(rta1)
	h2 := unique.Make(rta2)
	h3 := unique.Make(rta3)
	if h1 != h2 {
		t.Error("unique.Make should return same handle for equal values")
	}
	if h1 == h3 {
		t.Error("unique.Make should return different handle for different values")
	}
}

func TestRemoveRoutes(t *testing.T) {
	nr := func(peer uint32) route {
		return route{
			peer:       peer,
			nlri:       unique.Make(nlri{family: bgp.RF_IPv4_UC, path: 1}),
			nextHop:    unique.Make(netip.MustParseAddr("::ffff:198.51.100.8")),
			attributes: unique.Make(routeAttributes{asn: 65300}.ToComparable()),
			prefixLen:  96 + 24,
		}
	}
	t.Run("only route", func(t *testing.T) {
		r := newRIB()
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), nr(10))
		idx, _ := r.tree.Load().Lookup(netip.MustParseAddr("192.168.144.10"))
		count, empty := r.removeRoutes(idx, func(route) bool { return true }, true)
		if !empty {
			t.Error("removeRoutes() should have removed all routes from node")
		}
		if count != 1 {
			t.Error("removeRoutes() should have removed 1 route")
		}
		if diff := helpers.Diff(xsync.ToPlainMap(r.routes), map[routeKey]route{}); diff != "" {
			t.Errorf("removeRoutes() (-got, +want):\n%s", diff)
		}
	})

	t.Run("first route", func(t *testing.T) {
		r := newRIB()
		r1 := nr(10)
		r2 := nr(11)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r1)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r2)
		idx, _ := r.tree.Load().Lookup(netip.MustParseAddr("192.168.144.10"))
		count, empty := r.removeRoutes(idx, func(r route) bool { return r.peer == 10 }, true)
		if empty {
			t.Error("removeRoutes() should not have removed all routes from node")
		}
		if count != 1 {
			t.Error("removeRoutes() should have removed 1 route")
		}
		if diff := helpers.Diff(xsync.ToPlainMap(r.routes), map[routeKey]route{
			makeRouteKey(idx, 0): r2,
		}); diff != "" {
			t.Errorf("removeRoutes() (-got, +want):\n%s", diff)
		}
	})

	t.Run("second route", func(t *testing.T) {
		r := newRIB()
		r1 := nr(10)
		r2 := nr(11)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r1)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r2)
		idx, _ := r.tree.Load().Lookup(netip.MustParseAddr("192.168.144.10"))
		count, empty := r.removeRoutes(idx, func(r route) bool { return r.peer == 11 }, true)
		if empty {
			t.Error("removeRoutes() should not have removed all routes from node")
		}
		if count != 1 {
			t.Error("removeRoutes() should have removed 1 route")
		}
		if diff := helpers.Diff(xsync.ToPlainMap(r.routes), map[routeKey]route{
			makeRouteKey(idx, 0): r1,
		}); diff != "" {
			t.Errorf("removeRoutes() (-got, +want):\n%s", diff)
		}
	})
	t.Run("middle route", func(t *testing.T) {
		r := newRIB()
		r1 := nr(10)
		r2 := nr(11)
		r3 := nr(12)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r1)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r2)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r3)
		idx, _ := r.tree.Load().Lookup(netip.MustParseAddr("192.168.144.10"))
		count, empty := r.removeRoutes(idx, func(r route) bool { return r.peer == 11 }, true)
		if empty {
			t.Error("removeRoutes() should not have removed all routes from node")
		}
		if count != 1 {
			t.Error("removeRoutes() should have removed 1 route")
		}
		if diff := helpers.Diff(xsync.ToPlainMap(r.routes), map[routeKey]route{
			makeRouteKey(idx, 0): r1,
			makeRouteKey(idx, 1): r3,
		}); diff != "" {
			t.Errorf("removeRoutes() (-got, +want):\n%s", diff)
		}
	})
	t.Run("one route out of two", func(t *testing.T) {
		r := newRIB()
		r1 := nr(10)
		r2 := nr(11)
		r3 := nr(12)
		r4 := nr(13)
		r5 := nr(14)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r1)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r2)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r3)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r4)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r5)
		idx, _ := r.tree.Load().Lookup(netip.MustParseAddr("192.168.144.10"))
		count, empty := r.removeRoutes(idx, func(r route) bool { return r.peer%2 == 0 }, false)
		if empty {
			t.Error("removeRoutes() should not have removed all routes from node")
		}
		if count != 3 {
			t.Error("removeRoutes() should have removed 3 route")
		}
		if diff := helpers.Diff(xsync.ToPlainMap(r.routes), map[routeKey]route{
			makeRouteKey(idx, 0): r2,
			makeRouteKey(idx, 1): r4,
		}); diff != "" {
			t.Errorf("removeRoutes() (-got, +want):\n%s", diff)
		}
	})

	t.Run("all routes", func(t *testing.T) {
		r := newRIB()
		r1 := nr(10)
		r2 := nr(11)
		r3 := nr(12)
		r4 := nr(13)
		r5 := nr(14)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r1)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r2)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r3)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r4)
		r.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/120"), r5)
		idx, _ := r.tree.Load().Lookup(netip.MustParseAddr("192.168.144.10"))
		count, empty := r.removeRoutes(idx, func(route) bool { return true }, false)
		if !empty {
			t.Error("removeRoutes() should have removed all routes from node")
		}
		if count != 5 {
			t.Error("removeRoutes() should have removed 5 route")
		}
		if diff := helpers.Diff(xsync.ToPlainMap(r.routes), map[routeKey]route{}); diff != "" {
			t.Errorf("removeRoutes() (-got, +want):\n%s", diff)
		}
	})
}

func TestRIBHarness(t *testing.T) {
	for run := range 5 {
		random := rand.New(rand.NewPCG(uint64(run), 0))
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
			comment string
		}
		// We store all lookups that should succeed
		lookups := []lookup{}
		removeLookup := func(lookup lookup, comment string) {
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
				lookups[idx].comment = fmt.Sprintf("%s; %s", lookups[idx].comment, comment)
				break
			}
		}

		peers := []uint32{}
		for i := range totalExporters {
			for j := range peerPerExporter {
				peer := uint32((i << 16) + int(j))
				peers = append(peers, peer)
				toAdd := random.IntN(maxInitialRoutePerPeer)
				added := 0
				for range toAdd {
					lookup := lookup{
						peer: peer,
						addr: netip.MustParseAddr(fmt.Sprintf("2001:db8:f:%x::",
							random.IntN(300))),
						nextHop: netip.MustParseAddr(
							fmt.Sprintf("2001:db8:c::%x", random.IntN(500))),
						rd:      RD(random.IntN(3)),
						asn:     uint32(random.IntN(1000)),
						comment: "added during first pass",
					}
					added += r.AddPrefix(netip.PrefixFrom(lookup.addr, 64),
						route{
							peer:       peer,
							nlri:       unique.Make(nlri{rd: lookup.rd}),
							nextHop:    unique.Make(lookup.nextHop),
							attributes: unique.Make(routeAttributes{asn: lookup.asn}.ToComparable()),
						})
					removeLookup(lookup, fmt.Sprintf("erased by NH: %s, ASN: %d", lookup.nextHop, lookup.asn))
					lookups = append(lookups, lookup)
				}
				t.Logf("Run %d: added = %d/%d", run, added, toAdd)

				toRemove := random.IntN(maxRemovedRoutePerPeer)
				removed := 0
				for range toRemove {
					prefix := netip.MustParseAddr(fmt.Sprintf("2001:db8:f:%x::",
						random.IntN(300)))
					rd := RD(random.IntN(4))
					removed += r.RemovePrefix(netip.PrefixFrom(prefix, 64),
						route{
							peer: peer,
							nlri: unique.Make(nlri{
								rd: rd,
							}),
						})
					removeLookup(lookup{
						peer: peer,
						addr: prefix,
						rd:   rd,
					}, "removed during second pass")
				}
				t.Logf("Run %d: removed = %d/%d", run, removed, toRemove)

				toAdd = random.IntN(maxReaddedRoutePerPeer)
				added = 0
				for range toAdd {
					lookup := lookup{
						peer: peer,
						addr: netip.MustParseAddr(fmt.Sprintf("2001:db8:f:%x::",
							random.IntN(300))),
						nextHop: netip.MustParseAddr(
							fmt.Sprintf("2001:db8:c::%x", random.Uint32()%500)),
						asn:     uint32(random.IntN(1010)),
						comment: "added during third pass",
					}
					added += r.AddPrefix(netip.PrefixFrom(lookup.addr, 64),
						route{
							peer:       peer,
							nlri:       unique.Make(nlri{}),
							nextHop:    unique.Make(lookup.nextHop),
							attributes: unique.Make(routeAttributes{asn: lookup.asn}.ToComparable()),
						})
					removeLookup(lookup, fmt.Sprintf("erased by NH: %s, ASN: %d", lookup.nextHop, lookup.asn))
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
			prefixIdx, ok := r.tree.Load().Lookup(lookup.addr)
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
				if route.nextHop.Value() != lookup.nextHop || route.nlri.Value().rd != lookup.rd {
					continue
				}
				if route.attributes.Value().asn != lookup.asn {
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
				t.Errorf("cannot find %s for peer %d; NH: %s, RD: %s, ASN: %d, comment: %s",
					lookup.addr, lookup.peer,
					lookup.nextHop, lookup.rd, lookup.asn, lookup.comment)
				t.Logf("> available routes in tree for %s:", lookup.addr)
				for route := range r.iterateRoutesForPrefixIndex(prefixIdx) {
					t.Logf("  peer %d, NH: %s, RD: %s, ASN: %d",
						route.peer,
						route.nextHop.Value(),
						route.nlri.Value().rd, route.attributes.Value().asn)
				}
				t.Logf("> route history for prefix %s:", lookup.addr)
				for _, olookup := range lookups {
					if lookup.addr != olookup.addr {
						continue
					}
					t.Logf("  peer: %d, NH: %s, RD: %s, ASN: %d, comment: %s",
						olookup.peer, olookup.nextHop, olookup.rd, olookup.asn, olookup.comment)
				}
				if run == 1 {
					if testing.Verbose() {
						t.Log("> complete history:")
						for _, olookup := range lookups {
							t.Logf("  prefix: %s, peer: %d, NH: %s, RD: %s, ASN: %d comment: %s",
								olookup.addr,
								olookup.peer, olookup.nextHop, olookup.rd, olookup.asn, olookup.comment)
						}
					} else {
						t.Log("> complete history available in verbose mode")
					}
				}
			}
		}
		if removed < 5 {
			t.Error("did not remove more than 5 prefixes, suspicious...")
		}

		// Remove everything
		for _, peer := range peers {
			r.FlushPeer(peer)
		}

		if t.Failed() {
			break
		}
	}
}
