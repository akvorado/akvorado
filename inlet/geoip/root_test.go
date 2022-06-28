// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func copyFile(src string, dst string) {
	source, err := os.Open(src)
	if err != nil {
		panic(err)
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		panic(err)
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	if err != nil {
		panic(err)
	}
}

func TestDatabaseRefresh(t *testing.T) {
	dir := t.TempDir()
	config := DefaultConfiguration()
	config.CountryDatabase = filepath.Join(dir, "country.mmdb")
	config.ASNDatabase = filepath.Join(dir, "asn.mmdb")

	copyFile(filepath.Join("testdata", "GeoLite2-Country-Test.mmdb"),
		config.CountryDatabase)
	copyFile(filepath.Join("testdata", "GeoLite2-ASN-Test.mmdb"),
		config.ASNDatabase)

	r := reporter.NewMock(t)
	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	// Check we did load both databases
	gotMetrics := r.GetMetrics("akvorado_inlet_geoip_db_")
	expectedMetrics := map[string]string{
		`refresh_total{database="asn"}`:     "1",
		`refresh_total{database="country"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// Check we can reload the database
	copyFile(filepath.Join("testdata", "GeoLite2-Country-Test.mmdb"),
		filepath.Join(dir, "tmp.mmdb"))
	os.Rename(filepath.Join(dir, "tmp.mmdb"), config.CountryDatabase)
	time.Sleep(20 * time.Millisecond)
	gotMetrics = r.GetMetrics("akvorado_inlet_geoip_db_")
	expectedMetrics = map[string]string{
		`refresh_total{database="asn"}`:     "1",
		`refresh_total{database="country"}`: "2",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestStartWithoutDatabase(t *testing.T) {
	r := reporter.NewMock(t)
	c, err := New(r, DefaultConfiguration(), Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)
}

func TestStartWithMissingDatabase(t *testing.T) {
	countryConfiguration := DefaultConfiguration()
	countryConfiguration.CountryDatabase = "/i/do/not/exist"
	asnConfiguration := DefaultConfiguration()
	asnConfiguration.ASNDatabase = "/i/do/not/exist"
	cases := []struct {
		Name   string
		Config Configuration
	}{
		{"Inexisting country database", countryConfiguration},
		{"Inexisting ASN database", asnConfiguration},
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
