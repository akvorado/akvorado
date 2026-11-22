// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"net/netip"
	"testing"

	"akvorado/common/helpers"
	"akvorado/demoexporter/bmp"
	"akvorado/demoexporter/flowmodel"
)

func TestBMPRoutes(t *testing.T) {
	config := testConfiguration(t)
	meta := flowmodel.MockMetadata()
	mapping, err := newMapping(&config)
	if err != nil {
		t.Fatalf("newMapping() error:\n%+v", err)
	}

	got := bmpRoutes(&config, mapping, "exporter1", &meta)
	expected := []bmp.RouteConfiguration{
		{
			// Google traffic reaches exporter1 through the Telia transit.
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("216.58.206.0/24"),
				netip.MustParsePrefix("2a00:1450:4007::/48"),
			},
			ASPath: []uint32{1299, 15169},
		}, {
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("198.38.120.0/23"),
			},
			ASPath: []uint32{1299, 2906},
		}, {
			// The catch-all rule gives the default routes.
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("0.0.0.0/0"),
				netip.MustParsePrefix("::/0"),
			},
			ASPath: []uint32{1299},
		}, {
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("98.12.32.0/24"),
				netip.MustParsePrefix("192.0.2.0/24"),
				netip.MustParsePrefix("3fff:100::/48"),
			},
			ASPath: []uint32{64496},
		},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("bmpRoutes(exporter1) (-got, +want):\n%s", diff)
	}

	got = bmpRoutes(&config, mapping, "exporter2", &meta)
	expected = []bmp.RouteConfiguration{
		{
			// The neighbor AS is the origin AS, the path has one element.
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("216.58.206.0/24"),
				netip.MustParsePrefix("2a00:1450:4007::/48"),
			},
			ASPath: []uint32{15169},
		}, {
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("98.12.32.0/24"),
				netip.MustParsePrefix("192.0.2.0/24"),
				netip.MustParsePrefix("3fff:100::/48"),
			},
			ASPath: []uint32{64496},
		},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("bmpRoutes(exporter2) (-got, +want):\n%s", diff)
	}
}

func TestBMPRoutesDirection(t *testing.T) {
	// Google traffic only goes to exporter2 when it comes from the Internet.
	// The other way around, it follows the catch-all rule to exporter1, which
	// must advertise the prefixes too.
	config := testConfiguration(t)
	config.Mapping[0].Direction = MatchDirectionInbound
	config.Mapping[0].Assign = []AssignConfiguration{
		{Exporter: "exporter2", Interface: 11, Weight: 1},
	}
	meta := flowmodel.MockMetadata()
	mapping, err := newMapping(&config)
	if err != nil {
		t.Fatalf("newMapping() error:\n%+v", err)
	}

	got := bmpRoutes(&config, mapping, "exporter1", &meta)
	expected := []bmp.RouteConfiguration{
		{
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("216.58.206.0/24"),
				netip.MustParsePrefix("2a00:1450:4007::/48"),
			},
			ASPath: []uint32{1299, 15169},
		}, {
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("198.38.120.0/23"),
			},
			ASPath: []uint32{1299, 2906},
		}, {
			// The catch-all rule gives the default routes.
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("0.0.0.0/0"),
				netip.MustParsePrefix("::/0"),
			},
			ASPath: []uint32{1299},
		}, {
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("98.12.32.0/24"),
				netip.MustParsePrefix("192.0.2.0/24"),
				netip.MustParsePrefix("3fff:100::/48"),
			},
			ASPath: []uint32{64496},
		},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("bmpRoutes(exporter1) (-got, +want):\n%s", diff)
	}
}
