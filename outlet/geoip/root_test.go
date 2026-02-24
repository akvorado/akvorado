// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	source, err := os.Open(src)
	if err != nil {
		t.Fatalf("os.Open() error:\n%+v", err)
	}
	defer source.Close()

	destination, err := os.CreateTemp("", "tmp*.mmdb")
	if err != nil {
		t.Fatalf("os.CreateTemp() error:\n%+v", err)
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	if err != nil {
		t.Fatalf("io.Copy() error:\n%+v", err)
	}
	if err := os.Rename(destination.Name(), dst); err != nil {
		t.Fatalf("os.Rename() error:\n%+v", err)
	}
}

// waitForMetrics polls the database metrics until they match the expected ones.
// Databases are reloaded from a file watcher, so we don't know when this
// happens.
func waitForMetrics(t *testing.T, r *reporter.Reporter, expected map[string]string) {
	t.Helper()
	var diff string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		diff = helpers.Diff(r.GetMetrics("akvorado_outlet_geoip_db_"), expected)
		if diff == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Metrics (-got, +want):\n%s", diff)
}

func TestDatabaseRefresh(t *testing.T) {
	dir := t.TempDir()
	config := DefaultConfiguration()

	countryFile := filepath.Join(dir, "country.mmdb")
	asnFile := filepath.Join(dir, "asn.mmdb")
	config.GeoDatabase = []string{countryFile}
	config.ASNDatabase = []string{asnFile}

	copyFile(t, filepath.Join("testdata", "GeoLite2-Country-Test.mmdb"),
		countryFile)
	copyFile(t, filepath.Join("testdata", "GeoLite2-ASN-Test.mmdb"),
		asnFile)

	r := reporter.NewMock(t)
	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	// Check we did load both databases
	waitForMetrics(t, r, map[string]string{
		`refresh_total{database="asn"}`: "1",
		`refresh_total{database="geo"}`: "1",
	})

	// Check we can reload country database
	copyFile(t, filepath.Join("testdata", "GeoLite2-Country-Test.mmdb"), countryFile)
	waitForMetrics(t, r, map[string]string{
		`refresh_total{database="asn"}`: "1",
		`refresh_total{database="geo"}`: "2",
	})

	// Check we can reload ASN database
	copyFile(t, filepath.Join("testdata", "GeoLite2-ASN-Test.mmdb"), asnFile)
	waitForMetrics(t, r, map[string]string{
		`refresh_total{database="asn"}`: "2",
		`refresh_total{database="geo"}`: "2",
	})
}

func TestStartWithoutDatabase(t *testing.T) {
	r := reporter.NewMock(t)
	c, err := New(r, DefaultConfiguration(), Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	// Lookups return nothing instead of failing
	ip := netip.MustParseAddr("::ffff:1.0.0.1")
	if diff := helpers.Diff(c.LookupASN(ip), ASNInfo{}); diff != "" {
		t.Errorf("LookupASN() (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(c.LookupGeo(ip), GeoInfo{}); diff != "" {
		t.Errorf("LookupGeo() (-got, +want):\n%s", diff)
	}
}

func TestStartDatabaseOptional(t *testing.T) {
	dir := t.TempDir()
	config := DefaultConfiguration()

	countryFile := filepath.Join(dir, "country.mmdb")
	asnFile := filepath.Join(dir, "asn.mmdb")
	config.GeoDatabase = []string{countryFile}
	config.ASNDatabase = []string{asnFile}
	config.Optional = true

	r := reporter.NewMock(t)
	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	// Check we did not load anything
	gotMetrics := r.GetMetrics("akvorado_outlet_geoip_db_")
	expectedMetrics := map[string]string{}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	copyFile(t, filepath.Join("testdata", "GeoLite2-Country-Test.mmdb"),
		countryFile)
	copyFile(t, filepath.Join("testdata", "GeoLite2-ASN-Test.mmdb"),
		asnFile)

	// Check databases were loaded once they appeared
	waitForMetrics(t, r, map[string]string{
		`refresh_total{database="asn"}`: "1",
		`refresh_total{database="geo"}`: "1",
	})
}

func TestStartWithMissingDatabase(t *testing.T) {
	geoConfiguration := DefaultConfiguration()
	geoConfiguration.GeoDatabase = []string{"/i/do/not/exist"}
	asnConfiguration := DefaultConfiguration()
	asnConfiguration.ASNDatabase = []string{"/i/do/not/exist"}
	cases := []struct {
		Name   string
		Config Configuration
	}{
		{"inexisting geo database", geoConfiguration},
		{"inexisting ASN database", asnConfiguration},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			r := reporter.NewMock(t)
			c, err := New(r, tc.Config, Dependencies{Daemon: daemon.NewMock(t)})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			if err := c.Start(); err == nil {
				t.Fatalf("Start() got no error")
			}
		})
	}
}
