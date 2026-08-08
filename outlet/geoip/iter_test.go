// SPDX-FileCopyrightText: 2026 Free Mobile
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

// prefixContains tells if the prefix from a database contains the provided
// address. Both are mapped to IPv6 as a database may use either form.
func prefixContains(prefix netip.Prefix, ip string) bool {
	return helpers.PrefixTo6(prefix).Contains(helpers.AddrTo6(netip.MustParseAddr(ip)))
}

// iterASN walks all the ASN databases and returns, for each provided address,
// the ASN of the last prefix containing it.
func iterASN(t *testing.T, c *Component, ips []string) []ASNInfo {
	t.Helper()
	got := make([]ASNInfo, len(ips))
	c.IterASNDatabases(func(prefix netip.Prefix, info ASNInfo) {
		for i, ip := range ips {
			if prefixContains(prefix, ip) {
				got[i] = info
			}
		}
	})
	return got
}

// iterGeo walks all the geo databases and returns, for each provided address,
// the attributes of the last prefix containing it.
func iterGeo(t *testing.T, c *Component, ips []string) []GeoInfo {
	t.Helper()
	got := make([]GeoInfo, len(ips))
	c.IterGeoDatabases(func(prefix netip.Prefix, info GeoInfo) {
		for i, ip := range ips {
			if prefixContains(prefix, ip) {
				got[i] = info
			}
		}
	})
	return got
}

func TestIterASNDatabases(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, true)

	ips := []string{
		// ipinfo databases
		"2.19.4.138",
		"2a09:bac1:14a0:fd0::a:1",
		"213.248.218.137",
		// maxmind databases
		"1.0.0.0",
		"67.43.156.77",
		// not in any database
		"203.0.113.5",
	}
	expected := []ASNInfo{
		{ASNumber: 32787},
		{ASNumber: 13335},
		{ASNumber: 43519},
		{ASNumber: 15169},
		{ASNumber: 35908},
		{},
	}
	if diff := helpers.Diff(iterASN(t, c, ips), expected); diff != "" {
		t.Fatalf("IterASNDatabases() (-got, +want):\n%s", diff)
	}
}

func TestIterGeoDatabases(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, true)

	ips := []string{
		// ipinfo databases
		"1.0.84.10",
		"2.19.4.138",
		"2a09:bac1:14a0:fd0::a:1",
		"213.248.218.137",
		// maxmind databases
		"2.125.160.216",
		"2a02:ff00::1:1",
		"67.43.156.77",
		// not in any database
		"203.0.113.5",
	}
	expected := []GeoInfo{
		{Country: "JP", State: "Shimane", City: "Matsue"},
		{Country: "SG"},
		{Country: "CA"},
		{Country: "HK"},
		{Country: "GB", State: "ENG", City: "Boxford"},
		{Country: "IT"},
		{Country: "BT"},
		{},
	}
	if diff := helpers.Diff(iterGeo(t, c, ips), expected); diff != "" {
		t.Fatalf("IterGeoDatabases() (-got, +want):\n%s", diff)
	}
}

func TestIterNonExistingDatabases(t *testing.T) {
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

	// Databases which are not opened are skipped instead of failing
	ips := []string{"67.43.156.77"}
	if diff := helpers.Diff(iterASN(t, c, ips), []ASNInfo{{}}); diff != "" {
		t.Errorf("IterASNDatabases() (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(iterGeo(t, c, ips), []GeoInfo{{}}); diff != "" {
		t.Errorf("IterGeoDatabases() (-got, +want):\n%s", diff)
	}
}

func BenchmarkIterDatabases(b *testing.B) {
	r := reporter.NewMock(b)
	c := NewMock(b, r, true)

	b.Run("ASN", func(b *testing.B) {
		entries := 0
		for b.Loop() {
			c.IterASNDatabases(func(netip.Prefix, ASNInfo) {
				entries++
			})
		}
		b.ReportMetric(0, "ns/op")
		b.ReportMetric(float64(b.Elapsed())/float64(entries), "ns/entry")
	})

	b.Run("GeoIP", func(b *testing.B) {
		entries := 0
		for b.Loop() {
			c.IterGeoDatabases(func(netip.Prefix, GeoInfo) {
				entries++
			})
		}
		b.ReportMetric(0, "ns/op")
		b.ReportMetric(float64(b.Elapsed())/float64(entries), "ns/entry")
	})
}
