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
	"akvorado/inlet/routing/provider"

	"github.com/benbjohnson/clock"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
	"github.com/osrg/gobgp/v3/pkg/packet/bmp"
)

// NewMock creates a new mock provider for BMP (it's a real one
// listening to a random port).
func NewMock(t *testing.T, r *reporter.Reporter, conf provider.Configuration) (*Provider, *clock.Mock) {
	t.Helper()
	mockClock := clock.NewMock()
	confP := conf.(Configuration)
	confP.Listen = "127.0.0.1:0"
	p, err := confP.New(r, Dependencies{
		Daemon: daemon.NewMock(t),
		Clock:  mockClock,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	return p.(*Provider), mockClock
}

// PopulateRIB populates the RIB with a few entries.
func (p *Provider) PopulateRIB(t *testing.T) {
	t.Helper()
	p.active.Store(true)
	pinfo := p.addPeer(peerKey{
		exporter: netip.MustParseAddrPort("[::ffff:127.0.0.1]:47389"),
		ip:       netip.MustParseAddr("::ffff:203.0.113.4"),
		ptype:    bmp.BMP_PEER_TYPE_GLOBAL,
		asn:      64500,
	})
	p.rib.addPrefix(netip.MustParseAddr("::ffff:192.0.2.0"), 96+27, route{
		peer:    pinfo.reference,
		nlri:    p.rib.nlris.Put(nlri{family: bgp.RF_FS_IPv4_UC, path: 1}),
		nextHop: p.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:198.51.100.4"))),
		attributes: p.rib.rtas.Put(routeAttributes{
			asn:              174,
			asPath:           []uint32{64200, 1299, 174},
			communities:      []uint32{100, 200, 400},
			largeCommunities: []bgp.LargeCommunity{{ASN: 64200, LocalData1: 2, LocalData2: 3}},
			plen:             96 + 27,
		}),
	})
	p.rib.addPrefix(netip.MustParseAddr("::ffff:192.0.2.0"), 96+27, route{
		peer:    pinfo.reference,
		nlri:    p.rib.nlris.Put(nlri{family: bgp.RF_FS_IPv4_UC, path: 2}),
		nextHop: p.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:198.51.100.8"))),
		attributes: p.rib.rtas.Put(routeAttributes{
			asn:         174,
			asPath:      []uint32{64200, 174, 174, 174},
			communities: []uint32{100},
			plen:        96 + 27,
		}),
	})
	p.rib.addPrefix(netip.MustParseAddr("::ffff:192.0.2.128"), 96+27, route{
		peer:    pinfo.reference,
		nlri:    p.rib.nlris.Put(nlri{family: bgp.RF_FS_IPv4_UC}),
		nextHop: p.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:198.51.100.8"))),
		attributes: p.rib.rtas.Put(routeAttributes{
			asn:         1299,
			asPath:      []uint32{64200, 1299},
			communities: []uint32{500},
			plen:        96 + 27,
		}),
	})
	p.rib.addPrefix(netip.MustParseAddr("::ffff:1.0.0.0"), 96+24, route{
		peer:    pinfo.reference,
		nlri:    p.rib.nlris.Put(nlri{family: bgp.RF_FS_IPv4_UC}),
		nextHop: p.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:198.51.100.8"))),
		attributes: p.rib.rtas.Put(routeAttributes{
			asn:  65300,
			plen: 96 + 24,
		}),
	})
}

// LocalAddr returns the address the BMP collector is listening to.
func (p *Provider) LocalAddr() net.Addr {
	return p.address
}

// Reduce hash mask to generate collisions during tests (this should
// be optimized out by the compiler)
const rtaHashMask = 0xff

// MustParseRD parse a route distinguisher and panic on error.
func MustParseRD(input string) RD {
	var output RD
	if err := output.UnmarshalText([]byte(input)); err != nil {
		panic(err)
	}
	return output
}
