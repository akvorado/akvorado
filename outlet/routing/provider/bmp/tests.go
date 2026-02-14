// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package bmp

import (
	"net"
	"net/netip"
	"testing"
	"unique"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/outlet/routing/provider"

	"github.com/benbjohnson/clock"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"github.com/osrg/gobgp/v4/pkg/packet/bmp"
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
	newPeers := p.peers.Load().clone()
	pinfo := p.addPeer(newPeers, peerKey{
		exporter: netip.MustParseAddrPort("[::ffff:127.0.0.1]:47389"),
		ip:       netip.MustParseAddr("::ffff:203.0.113.4"),
		ptype:    bmp.BMP_PEER_TYPE_GLOBAL,
		asn:      64500,
	})
	p.peers.Store(newPeers)
	p.rib.AddPrefix(netip.MustParsePrefix("::ffff:192.0.2.0/123"), route{
		peer:    pinfo.reference,
		nlri:    unique.Make(nlri{family: bgp.RF_IPv4_UC, path: 1}),
		nextHop: unique.Make(netip.MustParseAddr("::ffff:198.51.100.4")),
		attributes: unique.Make(routeAttributes{
			asn:              174,
			asPath:           []uint32{64200, 1299, 174},
			communities:      []uint32{100, 200, 400},
			largeCommunities: []bgp.LargeCommunity{{ASN: 64200, LocalData1: 2, LocalData2: 3}},
		}.ToComparable()),
		prefixLen: 96 + 27,
	})
	p.rib.AddPrefix(netip.MustParsePrefix("::ffff:192.0.2.0/123"), route{
		peer:    pinfo.reference,
		nlri:    unique.Make(nlri{family: bgp.RF_IPv4_UC, path: 2}),
		nextHop: unique.Make(netip.MustParseAddr("::ffff:198.51.100.8")),
		attributes: unique.Make(routeAttributes{
			asn:         174,
			asPath:      []uint32{64200, 174, 174, 174},
			communities: []uint32{100},
		}.ToComparable()),
		prefixLen: 96 + 27,
	})
	p.rib.AddPrefix(netip.MustParsePrefix("::ffff:192.0.2.128/123"), route{
		peer:    pinfo.reference,
		nlri:    unique.Make(nlri{family: bgp.RF_IPv4_UC}),
		nextHop: unique.Make(netip.MustParseAddr("::ffff:198.51.100.8")),
		attributes: unique.Make(routeAttributes{
			asn:         1299,
			asPath:      []uint32{64200, 1299},
			communities: []uint32{500},
		}.ToComparable()),
		prefixLen: 96 + 27,
	})
	p.rib.AddPrefix(netip.MustParsePrefix("::ffff:1.0.0.0/120"), route{
		peer:       pinfo.reference,
		nlri:       unique.Make(nlri{family: bgp.RF_IPv4_UC}),
		nextHop:    unique.Make(netip.MustParseAddr("::ffff:198.51.100.8")),
		attributes: unique.Make(routeAttributes{asn: 65300}.ToComparable()),
		prefixLen:  96 + 24,
	})
	p.rib.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/117"), route{
		peer:    pinfo.reference,
		nlri:    unique.Make(nlri{rd: 10, family: bgp.RF_IPv4_UC, path: 0}),
		nextHop: unique.Make(netip.MustParseAddr("::ffff:203.0.113.14")),
		attributes: unique.Make(routeAttributes{
			asn:    1234,
			asPath: []uint32{54321, 1234},
		}.ToComparable()),
		prefixLen: 96 + 21,
	})
	p.rib.AddPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/118"), route{
		peer:    pinfo.reference,
		nlri:    unique.Make(nlri{rd: 10, family: bgp.RF_IPv4_UC, path: 0}),
		nextHop: unique.Make(netip.MustParseAddr("::ffff:203.0.113.15")),
		attributes: unique.Make(routeAttributes{
			asn:    1234,
			asPath: []uint32{1234},
		}.ToComparable()),
		prefixLen: 96 + 22,
	})
	p.rib.AddPrefix(netip.MustParsePrefix("::ffff:192.168.148.0/118"), route{
		peer:    pinfo.reference,
		nlri:    unique.Make(nlri{rd: 10, family: bgp.RF_IPv4_UC, path: 0}),
		nextHop: unique.Make(netip.MustParseAddr("::ffff:203.0.113.15")),
		attributes: unique.Make(routeAttributes{
			asn:    1234,
			asPath: []uint32{1234},
		}.ToComparable()),
		prefixLen: 96 + 22,
	})
	p.rib.AddPrefix(netip.MustParsePrefix("::ffff:192.168.148.1/128"), route{
		peer:    pinfo.reference,
		nlri:    unique.Make(nlri{rd: 10, family: bgp.RF_IPv4_UC, path: 0}),
		nextHop: unique.Make(netip.MustParseAddr("::ffff:203.0.113.14")),
		attributes: unique.Make(routeAttributes{
			asn:    1234,
			asPath: []uint32{1234},
		}.ToComparable()),
		prefixLen: 96 + 32,
	})
}

// LocalAddr returns the address the BMP collector is listening to.
func (p *Provider) LocalAddr() net.Addr {
	return p.address
}

// MustParseRD parse a route distinguisher and panic on error.
func MustParseRD(input string) RD {
	var output RD
	if err := output.UnmarshalText([]byte(input)); err != nil {
		panic(err)
	}
	return output
}

func init() {
	helpers.RegisterCmpOption(cmp.AllowUnexported(route{}))
	helpers.RegisterCmpOption(cmpopts.EquateComparable(
		unique.Handle[nlri]{},
		unique.Handle[netip.Addr]{},
		unique.Handle[routeAttributesComparable]{},
	))
}
