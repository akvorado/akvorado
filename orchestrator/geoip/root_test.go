// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func copyFile(t *testing.T, src string, dst string) {
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

	count := atomic.Uint32{}
	notify := c.Notify()
	go func() {
		for range notify {
			count.Add(1)
		}
	}()

	// Check we did load both databases
	gotMetrics := r.GetMetrics("akvorado_orchestrator_geoip_db_")
	expectedMetrics := map[string]string{
		`refresh_total{database="asn"}`: "1",
		`refresh_total{database="geo"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	time.Sleep(10 * time.Millisecond)
	if current := count.Load(); current != 1 {
		t.Errorf("Notified %d times instead of %d", current, 1)
	}

	// Check we can reload country database
	copyFile(t, filepath.Join("testdata", "GeoLite2-Country-Test.mmdb"), countryFile)
	time.Sleep(20 * time.Millisecond)
	gotMetrics = r.GetMetrics("akvorado_orchestrator_geoip_db_")
	expectedMetrics = map[string]string{
		`refresh_total{database="asn"}`: "1",
		`refresh_total{database="geo"}`: "2",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	if current := count.Load(); current != 2 {
		t.Errorf("Notified %d times instead of %d", current, 2)
	}

	// Check we can reload ASN database
	copyFile(t, filepath.Join("testdata", "GeoLite2-ASN-Test.mmdb"), asnFile)
	time.Sleep(20 * time.Millisecond)
	gotMetrics = r.GetMetrics("akvorado_orchestrator_geoip_db_")
	expectedMetrics = map[string]string{
		`refresh_total{database="asn"}`: "2",
		`refresh_total{database="geo"}`: "2",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	if current := count.Load(); current != 3 {
		t.Errorf("Notified %d times instead of %d", current, 3)
	}
}

func TestStartWithoutDatabase(t *testing.T) {
	r := reporter.NewMock(t)
	c, err := New(r, DefaultConfiguration(), Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	count := atomic.Uint32{}
	notify := c.Notify()
	go func() {
		for range notify {
			count.Add(1)
		}
	}()

	time.Sleep(10 * time.Millisecond)
	if current := count.Load(); current != 1 {
		t.Errorf("Notified %d times instead of %d", current, 1)
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

	count := atomic.Uint32{}
	notify := c.Notify()
	go func() {
		for range notify {
			count.Add(1)
		}
	}()

	// Check we did not load anything
	gotMetrics := r.GetMetrics("akvorado_orchestrator_geoip_db_")
	expectedMetrics := map[string]string{}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	time.Sleep(10 * time.Millisecond)
	if current := count.Load(); current != 1 {
		t.Errorf("Notified %d times instead of %d", current, 1)
	}

	copyFile(t, filepath.Join("testdata", "GeoLite2-Country-Test.mmdb"),
		countryFile)
	copyFile(t, filepath.Join("testdata", "GeoLite2-ASN-Test.mmdb"),
		asnFile)

	// Check databases were loaded
	time.Sleep(50 * time.Millisecond)
	gotMetrics = r.GetMetrics("akvorado_orchestrator_geoip_db_")
	expectedMetrics = map[string]string{
		`refresh_total{database="asn"}`: "1",
		`refresh_total{database="geo"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	if current := count.Load(); current != 3 && current != 2 {
		t.Errorf("Notified %d times instead of 2 or 3", current)
	}
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
		{"Inexisting geo database", geoConfiguration},
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
