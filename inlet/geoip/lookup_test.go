// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"net/netip"
	"path/filepath"
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestLookup(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r)

	cases := []struct {
		IP              string
		ExpectedASN     uint32
		ExpectedCountry string
	}{
		{
			IP:          "1.0.0.0",
			ExpectedASN: 15169,
		}, {
			IP:          "::ffff:1.0.0.0",
			ExpectedASN: 15169,
		}, {
			IP:              "2.125.160.216",
			ExpectedCountry: "GB",
		}, {
			IP:              "2a02:ff00::1:1",
			ExpectedCountry: "IT",
		}, {
			IP:              "67.43.156.77",
			ExpectedASN:     35908,
			ExpectedCountry: "BT",
		},
	}
	for _, tc := range cases {
		gotCountry := c.LookupCountry(netip.MustParseAddr(tc.IP))
		if diff := helpers.Diff(gotCountry, tc.ExpectedCountry); diff != "" {
			t.Errorf("LookupCountry(%q) (-got, +want):\n%s", tc.IP, diff)
		}
		gotASN := c.LookupASN(netip.MustParseAddr(tc.IP))
		if diff := helpers.Diff(gotASN, tc.ExpectedASN); diff != "" {
			t.Errorf("LookupASN(%q) (-got, +want):\n%s", tc.IP, diff)
		}
	}
	gotMetrics := r.GetMetrics("akvorado_inlet_geoip_")
	expectedMetrics := map[string]string{
		`db_hits_total{database="asn"}`:    "3",
		`db_hits_total{database="geo"}`:    "3",
		`db_misses_total{database="asn"}`:  "2",
		`db_misses_total{database="geo"}`:  "2",
		`db_refresh_total{database="asn"}`: "1",
		`db_refresh_total{database="geo"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestLookupIPInfo(t *testing.T) {
	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	// The JSON version of this one is here:
	// https://github.com/ipinfo/sample-database/blob/main/IP%20to%20Country%20ASN/ip_country_asn_sample.json
	config.GeoDatabase = filepath.Join("testdata", "ip_country_asn_sample.mmdb")
	config.ASNDatabase = filepath.Join("testdata", "ip_country_asn_sample.mmdb")
	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+s", err)
	}
	helpers.StartStop(t, c)

	cases := []struct {
		IP              string
		ExpectedASN     uint32
		ExpectedCountry string
	}{
		{
			IP:              "2.19.4.138",
			ExpectedASN:     32787,
			ExpectedCountry: "SG",
		}, {
			IP:              "2a09:bac1:14a0:fd0::a:1",
			ExpectedASN:     13335,
			ExpectedCountry: "CA",
		}, {
			IP:              "213.248.218.137",
			ExpectedASN:     43519,
			ExpectedCountry: "HK",
		},
	}
	for _, tc := range cases {
		gotCountry := c.LookupCountry(netip.MustParseAddr(tc.IP))
		if diff := helpers.Diff(gotCountry, tc.ExpectedCountry); diff != "" {
			t.Errorf("LookupCountry(%q) (-got, +want):\n%s", tc.IP, diff)
		}
		gotASN := c.LookupASN(netip.MustParseAddr(tc.IP))
		if diff := helpers.Diff(gotASN, tc.ExpectedASN); diff != "" {
			t.Errorf("LookupASN(%q) (-got, +want):\n%s", tc.IP, diff)
		}
	}
}
