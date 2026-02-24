// SPDX-FileCopyrightText: 2024 Free Mobile
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

func BenchmarkLookup(b *testing.B) {
	r := reporter.NewMock(b)
	c := NewMock(b, r, true)
	ip := netip.MustParseAddr("::ffff:67.43.156.77")

	b.Run("ASN", func(b *testing.B) {
		for b.Loop() {
			c.LookupASN(ip)
		}
	})

	b.Run("GeoIP", func(b *testing.B) {
		for b.Loop() {
			c.LookupGeo(ip)
		}
	})
}

func TestLookupGeo(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, true)

	cases := []struct {
		IP              string
		ExpectedCountry string
		ExpectedState   string
		ExpectedCity    string
	}{
		// ipinfo databases
		{"::ffff:1.0.84.10", "JP", "Shimane", "Matsue"},
		{"::ffff:2.19.4.138", "SG", "", ""},
		{"::ffff:213.248.218.137", "HK", "", ""},
		// maxmind databases
		{"::ffff:2.125.160.216", "GB", "ENG", "Boxford"},
		{"2a02:ff00::1:1", "IT", "", ""},
		{"::ffff:67.43.156.77", "BT", "", ""},
		// not in any database
		{"::ffff:203.0.113.5", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.IP, func(t *testing.T) {
			ip := netip.MustParseAddr(tc.IP)
			got := c.LookupGeo(ip)
			expected := GeoInfo{
				Country: tc.ExpectedCountry,
				State:   tc.ExpectedState,
				City:    tc.ExpectedCity,
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("LookupGeo(%s) (-got, +want):\n%s", tc.IP, diff)
			}
		})
	}
}

func TestLookupASN(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, true)

	cases := []struct {
		IP          string
		ExpectedASN uint32
	}{
		// ipinfo databases
		{"::ffff:2.19.4.138", 32787},
		{"2a09:bac1:14a0:fd0::a:1", 13335},
		{"::ffff:213.248.218.137", 43519},
		// maxmind databases
		{"::ffff:1.0.0.0", 15169},
		{"::ffff:67.43.156.77", 35908},
		// not in any database
		{"::ffff:203.0.113.5", 0},
	}
	for _, tc := range cases {
		t.Run(tc.IP, func(t *testing.T) {
			ip := netip.MustParseAddr(tc.IP)
			got := c.LookupASN(ip)
			expected := ASNInfo{ASNumber: tc.ExpectedASN}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("LookupASN(%s) (-got, +want):\n%s", tc.IP, diff)
			}
		})
	}
}

func TestLookupNonExistingDatabase(t *testing.T) {
	dir := t.TempDir()
	config := DefaultConfiguration()
	config.GeoDatabase = append(config.GeoDatabase, filepath.Join(dir, "1.mmdb"))
	config.ASNDatabase = append(config.ASNDatabase, filepath.Join(dir, "2.mmdb"))
	config.Optional = true

	r := reporter.NewMock(t)
	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	ip := netip.MustParseAddr("::ffff:67.43.156.77")
	if diff := helpers.Diff(c.LookupASN(ip), ASNInfo{}); diff != "" {
		t.Errorf("LookupASN() (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(c.LookupGeo(ip), GeoInfo{}); diff != "" {
		t.Errorf("LookupGeo() (-got, +want):\n%s", diff)
	}
}
