//go:build !release

package geoip

import (
	"path/filepath"
	"testing"

	"akvorado/daemon"
	"akvorado/reporter"
)

// NewMock creates a GeoIP component usable for testing. It is already
// started. It panics if there is an issue. Data of both databases are
// available here:
//  - https://github.com/maxmind/MaxMind-DB/blob/main/source-data/GeoLite2-ASN-Test.json
//  - https://github.com/maxmind/MaxMind-DB/blob/main/source-data/GeoLite2-Country-Test.json
func NewMock(t *testing.T, r *reporter.Reporter) *Component {
	t.Helper()
	config := DefaultConfiguration
	config.CountryDatabase = filepath.Join("..", "geoip", "testdata", "GeoLite2-Country-Test.mmdb")
	config.ASNDatabase = filepath.Join("..", "geoip", "testdata", "GeoLite2-ASN-Test.mmdb")
	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+s", err)
	}
	if err := c.Start(); err != nil {
		t.Fatalf("Start() error:\n%+s", err)
	}
	return c
}
