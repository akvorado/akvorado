// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package bmp

import (
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/outlet/routing/provider"

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
	p.rib.addPrefix(netip.MustParsePrefix("::ffff:192.0.2.0/123"), route{
		peer:    pinfo.reference,
		nlri:    p.rib.nlris.Put(nlri{family: bgp.RF_IPv4_UC, path: 1}),
		nextHop: p.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:198.51.100.4"))),
		attributes: p.rib.rtas.Put(routeAttributes{
			asn:              174,
			asPath:           []uint32{64200, 1299, 174},
			communities:      []uint32{100, 200, 400},
			largeCommunities: []bgp.LargeCommunity{{ASN: 64200, LocalData1: 2, LocalData2: 3}},
		}),
		prefixLen: 96 + 27,
	})
	p.rib.addPrefix(netip.MustParsePrefix("::ffff:192.0.2.0/123"), route{
		peer:    pinfo.reference,
		nlri:    p.rib.nlris.Put(nlri{family: bgp.RF_IPv4_UC, path: 2}),
		nextHop: p.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:198.51.100.8"))),
		attributes: p.rib.rtas.Put(routeAttributes{
			asn:         174,
			asPath:      []uint32{64200, 174, 174, 174},
			communities: []uint32{100},
		}),
		prefixLen: 96 + 27,
	})
	p.rib.addPrefix(netip.MustParsePrefix("::ffff:192.0.2.128/123"), route{
		peer:    pinfo.reference,
		nlri:    p.rib.nlris.Put(nlri{family: bgp.RF_IPv4_UC}),
		nextHop: p.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:198.51.100.8"))),
		attributes: p.rib.rtas.Put(routeAttributes{
			asn:         1299,
			asPath:      []uint32{64200, 1299},
			communities: []uint32{500},
		}),
		prefixLen: 96 + 27,
	})
	p.rib.addPrefix(netip.MustParsePrefix("::ffff:1.0.0.0/120"), route{
		peer:    pinfo.reference,
		nlri:    p.rib.nlris.Put(nlri{family: bgp.RF_IPv4_UC}),
		nextHop: p.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:198.51.100.8"))),
		attributes: p.rib.rtas.Put(routeAttributes{
			asn: 65300,
		}),
		prefixLen: 96 + 24,
	})
	p.rib.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/117"), route{
		peer:    pinfo.reference,
		nlri:    p.rib.nlris.Put(nlri{rd: 10, family: bgp.RF_IPv4_UC, path: 0}),
		nextHop: p.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:203.0.113.14"))),
		attributes: p.rib.rtas.Put(routeAttributes{
			asn:    1234,
			asPath: []uint32{54321, 1234},
		}),
		prefixLen: 96 + 21,
	})
	p.rib.addPrefix(netip.MustParsePrefix("::ffff:192.168.144.0/118"), route{
		peer:    pinfo.reference,
		nlri:    p.rib.nlris.Put(nlri{rd: 10, family: bgp.RF_IPv4_UC, path: 0}),
		nextHop: p.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:203.0.113.15"))),
		attributes: p.rib.rtas.Put(routeAttributes{
			asn:    1234,
			asPath: []uint32{1234},
		}),
		prefixLen: 96 + 22,
	})
	p.rib.addPrefix(netip.MustParsePrefix("::ffff:192.168.148.0/118"), route{
		peer:    pinfo.reference,
		nlri:    p.rib.nlris.Put(nlri{rd: 10, family: bgp.RF_IPv4_UC, path: 0}),
		nextHop: p.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:203.0.113.15"))),
		attributes: p.rib.rtas.Put(routeAttributes{
			asn:    1234,
			asPath: []uint32{1234},
		}),
		prefixLen: 96 + 22,
	})
	p.rib.addPrefix(netip.MustParsePrefix("::ffff:192.168.148.1/128"), route{
		peer:    pinfo.reference,
		nlri:    p.rib.nlris.Put(nlri{rd: 10, family: bgp.RF_IPv4_UC, path: 0}),
		nextHop: p.rib.nextHops.Put(nextHop(netip.MustParseAddr("::ffff:203.0.113.14"))),
		attributes: p.rib.rtas.Put(routeAttributes{
			asn:    1234,
			asPath: []uint32{1234},
		}),
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
	helpers.AddPrettyFormatter(reflect.TypeOf(route{}), fmt.Sprint)
}
