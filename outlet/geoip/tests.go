// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package geoip

import (
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// TestDataPath returns the path of one of the test databases. It can be used
// from other components.
func TestDataPath(database string) string {
	_, src, _, _ := runtime.Caller(0)
	return filepath.Join(path.Dir(src), "testdata", database)
}

// NewMock creates a GeoIP component usable for testing. It is already
// started. It panics if there is an issue. Data of both databases are
// available here:
//   - https://github.com/maxmind/MaxMind-DB/blob/main/source-data/GeoLite2-ASN-Test.json
//   - https://github.com/maxmind/MaxMind-DB/blob/main/source-data/GeoLite2-Country-Test.json
func NewMock(t testing.TB, r *reporter.Reporter, withData bool) *Component {
	t.Helper()
	config := DefaultConfiguration()
	if withData {
		config.GeoDatabase = []string{
			TestDataPath("GeoLite2-City-Test.mmdb"),
			TestDataPath("ip_country_asn_sample.mmdb"),
			TestDataPath("ip_geolocation_sample.mmdb"),
		}
		config.ASNDatabase = []string{
			TestDataPath("GeoLite2-ASN-Test.mmdb"),
			TestDataPath("ip_country_asn_sample.mmdb"),
		}
	}
	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+s", err)
	}
	helpers.StartStop(t, c)
	return c
}
