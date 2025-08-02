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
	clickhouseComponent := clickhousedb.SetupClickHouse(t, r, false)

	t.Run("only GeoIP", func(t *testing.T) {
		// First use only GeoIP
		c, err := New(r, config, Dependencies{
			Daemon:     daemon.NewMock(t),
			HTTP:       httpserver.NewMock(t, r),
			Schema:     schema.NewMock(t),
			GeoIP:      geoip.NewMock(t, r, true),
			ClickHouse: clickhouseComponent,
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
					"1.0.0.0/24,,,,,AU,Queensland,Brisbane,,15169",
					"1.0.1.0/24,,,,,CN,Fujian,Xiamen,,",
					"1.0.2.0/23,,,,,CN,Fujian,Xiamen,,",
					"1.0.4.0/22,,,,,AU,Victoria,Melbourne,,",
					"1.0.8.0/21,,,,,CN,Guangdong,Shenzhen,,",
					"1.0.16.0/29,,,,,JP,Tokyo,Tokyo,,",
					"1.0.16.8/30,,,,,JP,Tokyo,Tokyo,,",
					"1.0.16.12/31,,,,,JP,Tokyo,Tokyo,,",
					"1.0.16.14/32,,,,,JP,Tokyo,Asagaya-minami,,",
					"1.0.16.15/32,,,,,JP,Tokyo,Tokyo,,",
					"1.0.16.16/28,,,,,JP,Tokyo,Tokyo,,",
					"1.0.16.32/27,,,,,JP,Tokyo,Tokyo,,",
					"1.0.16.64/26,,,,,JP,Tokyo,Tokyo,,",
					"1.0.16.128/25,,,,,JP,Tokyo,Tokyo,,",
					"1.0.17.0/24,,,,,JP,Tokyo,Asagaya-minami,,",
					"1.0.18.0/23,,,,,JP,Tokyo,Asagaya-minami,,",
				},
			},
		})
	})

	t.Run("custom networks", func(t *testing.T) {
		// Second use: add custom networks
		config.Networks = helpers.MustNewSubnetMap(map[string]NetworkAttributes{
			"::ffff:0.80.0.0/112":  {Tenant: "Alfred"}, // not covered by GeoIP
			"::ffff:1.0.0.0/116":   {Name: "infra"},    // not covered by GeoIP but covers GeoIP entries
			"::ffff:1.0.16.64/122": {Name: "infra"},    // matching a GeoIP entry
			"::ffff:1.0.16.66/128": {Tenant: "Alfred"}, // nested in previous one
		})

		c, err := New(r, config, Dependencies{
			Daemon:     daemon.NewMock(t),
			HTTP:       httpserver.NewMock(t, r),
			Schema:     schema.NewMock(t),
			GeoIP:      geoip.NewMock(t, r, true),
			ClickHouse: clickhouseComponent,
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
					"0.80.0.0/16,,,,,,,,Alfred,",                        // not covered by GeoIP
					"1.0.0.0/20,infra,,,,,,,,",                          // not covered by GeoIP...
					"1.0.0.0/24,infra,,,,AU,Queensland,Brisbane,,15169", // but covers GeoIP entries
					"1.0.1.0/24,infra,,,,CN,Fujian,Xiamen,,",            // but covers GeoIP entries
					"1.0.2.0/23,infra,,,,CN,Fujian,Xiamen,,",            // but covers GeoIP entries
					"1.0.4.0/22,infra,,,,AU,Victoria,Melbourne,,",       // but covers GeoIP entries
					"1.0.8.0/21,infra,,,,CN,Guangdong,Shenzhen,,",       // but covers GeoIP entries
					"1.0.16.0/29,,,,,JP,Tokyo,Tokyo,,",
					"1.0.16.8/30,,,,,JP,Tokyo,Tokyo,,",
					"1.0.16.12/31,,,,,JP,Tokyo,Tokyo,,",
					"1.0.16.14/32,,,,,JP,Tokyo,Asagaya-minami,,",
					"1.0.16.15/32,,,,,JP,Tokyo,Tokyo,,",
					"1.0.16.16/28,,,,,JP,Tokyo,Tokyo,,",
					"1.0.16.32/27,,,,,JP,Tokyo,Tokyo,,",
					"1.0.16.64/26,infra,,,,JP,Tokyo,Tokyo,,",       // matching a GeoIP entry
					"1.0.16.66/32,infra,,,,JP,Tokyo,Tokyo,Alfred,", // nested in previous one
					"1.0.16.128/25,,,,,JP,Tokyo,Tokyo,,",
					"1.0.17.0/24,,,,,JP,Tokyo,Asagaya-minami,,",
					"1.0.18.0/23,,,,,JP,Tokyo,Asagaya-minami,,",
					"1.0.20.0/22,,,,,JP,Tokyo,Asagaya-minami,,",
					"1.0.24.0/21,,,,,JP,Tokyo,Asagaya-minami,,",
					"1.0.32.0/19,,,,,CN,Guangdong,Shenzhen,,",
					"1.0.64.0/20,,,,,JP,Hiroshima,Hiroshima,,",
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
			ClickHouse: clickhouseComponent,
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
