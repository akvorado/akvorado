// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"os"
	"path/filepath"
	"testing"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/orchestrator/geoip"
)

func TestNetworksCSVWithGeoip(t *testing.T) {
	config := DefaultConfiguration()
	config.SkipMigrations = true
	r := reporter.NewMock(t)
	clickHouseComponent := clickhousedb.SetupClickHouse(t, r, false)

	t.Run("only GeoIP", func(t *testing.T) {
		// First use only GeoIP
		c, err := New(r, config, Dependencies{
			Daemon:     daemon.NewMock(t),
			HTTP:       httpserver.NewMock(t, r),
			Schema:     schema.NewMock(t),
			GeoIP:      geoip.NewMock(t, r, true),
			ClickHouse: clickHouseComponent,
		})
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
		helpers.StartStop(t, c)

		helpers.TestHTTPEndpoints(t, c.d.HTTP.LocalAddr(), helpers.HTTPEndpointCases{
			{
				Description: "networks.csv",
				URL:         "/api/v0/orchestrator/clickhouse/networks.csv",
				ContentType: "text/csv; charset=utf-8",
				FirstLines: []string{
					"network,name,role,site,region,country,state,city,tenant,asn",
					"1.0.0.0/24,,,,,,,,,15169",
					"1.128.0.0/11,,,,,,,,,1221",
					"2.19.4.136/30,,,,,SG,,,,32787",
					"2.19.4.140/32,,,,,SG,,,,32787",
					"2.125.160.216/29,,,,,GB,,,,",
					"12.81.92.0/22,,,,,,,,,7018",
					"12.81.96.0/19,,,,,,,,,7018",
					"12.81.128.0/17,,,,,,,,,7018",
					"12.82.0.0/15,,,,,,,,,7018",
					"12.84.0.0/14,,,,,,,,,7018",
					"12.88.0.0/13,,,,,,,,,7018",
					"12.96.0.0/20,,,,,,,,,7018",
					"12.96.16.0/24,,,,,,,,,7018",
					"15.0.0.0/8,,,,,,,,,71",
					"16.0.0.0/8,,,,,,,,,71",
					"18.0.0.0/8,,,,,,,,,3",
				},
			},
		})
	})

	t.Run("custom networks", func(t *testing.T) {
		// Second use: add custom networks
		config.Networks = helpers.MustNewSubnetMap(map[string]NetworkAttributes{
			"::ffff:12.80.0.0/112":  {Name: "infra"},    // not covered by GeoIP
			"::ffff:12.81.96.0/115": {Name: "infra"},    // matching a GeoIP entry
			"::ffff:12.81.96.0/120": {Tenant: "Alfred"}, // nested in previous one
			"::ffff:14.0.0.0/103":   {Tenant: "Alfred"}, // not covered by GeoIP but covers GeoIP entries
		})

		c, err := New(r, config, Dependencies{
			Daemon:     daemon.NewMock(t),
			HTTP:       httpserver.NewMock(t, r),
			Schema:     schema.NewMock(t),
			GeoIP:      geoip.NewMock(t, r, true),
			ClickHouse: clickHouseComponent,
		})
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
		helpers.StartStop(t, c)
		helpers.TestHTTPEndpoints(t, c.d.HTTP.LocalAddr(), helpers.HTTPEndpointCases{
			{
				Description: "networks.csv",
				URL:         "/api/v0/orchestrator/clickhouse/networks.csv",
				ContentType: "text/csv; charset=utf-8",
				FirstLines: []string{
					"network,name,role,site,region,country,state,city,tenant,asn",
					"1.0.0.0/24,,,,,,,,,15169",
					"1.128.0.0/11,,,,,,,,,1221",
					"2.19.4.136/30,,,,,SG,,,,32787",
					"2.19.4.140/32,,,,,SG,,,,32787",
					"2.125.160.216/29,,,,,GB,,,,",
					"12.80.0.0/16,infra,,,,,,,,", // not covered by GeoIP
					"12.81.92.0/22,,,,,,,,,7018",
					"12.81.96.0/19,infra,,,,,,,,7018",       // matching a GeoIP entry
					"12.81.96.0/24,infra,,,,,,,Alfred,7018", // nested in previous one
					"12.81.128.0/17,,,,,,,,,7018",
					"12.82.0.0/15,,,,,,,,,7018",
					"12.84.0.0/14,,,,,,,,,7018",
					"12.88.0.0/13,,,,,,,,,7018",
					"12.96.0.0/20,,,,,,,,,7018",
					"12.96.16.0/24,,,,,,,,,7018",
					"14.0.0.0/7,,,,,,,,Alfred,",   // not covered by GeoIP
					"15.0.0.0/8,,,,,,,,Alfred,71", // but covers GeoIP entries
					"16.0.0.0/8,,,,,,,,,71",
					"18.0.0.0/8,,,,,,,,,3",
				},
			},
		})
	})

	t.Run("cleanup old files", func(t *testing.T) {
		_, err := os.CreateTemp("", networksCSVPattern)
		if err != nil {
			t.Fatalf("os.CreateTemp() error:\n%+v", err)
		}
		c, err := New(r, config, Dependencies{
			Daemon:     daemon.NewMock(t),
			HTTP:       httpserver.NewMock(t, r),
			Schema:     schema.NewMock(t),
			GeoIP:      geoip.NewMock(t, r, true),
			ClickHouse: clickHouseComponent,
		})
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
		helpers.StartStop(t, c)

		// HTTP request to ensure we are ready
		helpers.TestHTTPEndpoints(t, c.d.HTTP.LocalAddr(), helpers.HTTPEndpointCases{
			{
				Description: "networks.csv",
				URL:         "/api/v0/orchestrator/clickhouse/networks.csv",
				ContentType: "text/csv; charset=utf-8",
				FirstLines: []string{
					"network,name,role,site,region,country,state,city,tenant,asn",
				},
			},
		})

		// Clean up old files
		got, err := filepath.Glob(filepath.Join(os.TempDir(), networksCSVPattern))
		if err != nil {
			t.Fatalf("filepath.Glob() error:\n%+v", err)
		}
		c.networksCSVLock.Lock()
		expected := []string{c.networksCSVFile.Name()}
		c.networksCSVLock.Unlock()

		if diff := helpers.Diff(got, expected); diff != "" {
			t.Fatalf("Temporary files (-got, +want):\n%s", diff)
		}
	})

}
