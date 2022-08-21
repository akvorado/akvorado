// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package bmp

import (
	"net"
	"net/netip"
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/reporter"

	"github.com/benbjohnson/clock"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
	"github.com/osrg/gobgp/v3/pkg/packet/bmp"
)

// NewMock creates a new mock component for BMP (it's a real one
// listening to a random port).
func NewMock(t *testing.T, r *reporter.Reporter, conf Configuration) (*Component, *clock.Mock) {
	t.Helper()
	mockClock := clock.NewMock()
	conf.Listen = "127.0.0.1:0"
	c, err := New(r, conf, Dependencies{
		Daemon: daemon.NewMock(t),
		Clock:  mockClock,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	return c, mockClock
}

// PopulateRIB populates the RIB with a few entries.
func (c *Component) PopulateRIB(t *testing.T) {
	t.Helper()
	pinfo := c.addPeer(peerKey{
		exporter: netip.MustParseAddrPort("[::ffff:127.0.0.1]:47389"),
		ip:       netip.MustParseAddr("::ffff:203.0.113.4"),
		ptype:    bmp.BMP_PEER_TYPE_GLOBAL,
		asn:      64500,
	})
	c.rib.addPrefix(netip.MustParseAddr("::ffff:192.0.2.0"), 96+27, route{
		peer:    pinfo.reference,
		nlri:    nlri{family: bgp.RF_FS_IPv4_UC, path: 1},
		nextHop: c.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:198.51.100.4"))),
		attributes: c.rib.rtas.Put(routeAttributes{
			asn:              174,
			asPath:           []uint32{64200, 1299, 174},
			communities:      []uint32{100, 200, 400},
			largeCommunities: []bgp.LargeCommunity{{ASN: 64200, LocalData1: 2, LocalData2: 3}},
		}),
	})
	c.rib.addPrefix(netip.MustParseAddr("::ffff:192.0.2.0"), 96+27, route{
		peer:    pinfo.reference,
		nlri:    nlri{family: bgp.RF_FS_IPv4_UC, path: 2},
		nextHop: c.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:198.51.100.8"))),
		attributes: c.rib.rtas.Put(routeAttributes{
			asn:         174,
			asPath:      []uint32{64200, 174, 174, 174},
			communities: []uint32{100},
		}),
	})
	c.rib.addPrefix(netip.MustParseAddr("::ffff:192.0.2.128"), 96+27, route{
		peer:    pinfo.reference,
		nlri:    nlri{family: bgp.RF_FS_IPv4_UC},
		nextHop: c.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:198.51.100.8"))),
		attributes: c.rib.rtas.Put(routeAttributes{
			asn:         1299,
			asPath:      []uint32{64200, 1299},
			communities: []uint32{500},
		}),
	})
	c.rib.addPrefix(netip.MustParseAddr("::ffff:1.0.0.0"), 96+24, route{
		peer:    pinfo.reference,
		nlri:    nlri{family: bgp.RF_FS_IPv4_UC},
		nextHop: c.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:198.51.100.8"))),
		attributes: c.rib.rtas.Put(routeAttributes{
			asn: 65300,
		}),
	})
}

// LocalAddr returns the address the BMP collector is listening to.
func (c *Component) LocalAddr() net.Addr {
	return c.address
}

// Reduce hash mask to generate collisions during tests (this should
// be optimized out by the compiler)
const rtaHashMask = 0xff

// Use a predictable seed for tests.
var rtaHashSeed = uint64(0)

// MustParseRD parse a route distinguisher and panic on error.
func MustParseRD(input string) RD {
	var output RD
	if err := output.UnmarshalText([]byte(input)); err != nil {
		panic(err)
	}
	return output
}
