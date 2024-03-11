// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"testing"
	"time"

	"akvorado/orchestrator/clickhouse/geoip"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

func TestNetworkGeoip(t *testing.T) {
	config := DefaultConfiguration()
	config.SkipMigrations = true
	r := reporter.NewMock(t)
	clickHouseComponent := clickhousedb.SetupClickHouse(t, r)

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

	time.Sleep(1000 * time.Millisecond)
	helpers.TestHTTPEndpoints(t, c.d.HTTP.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "try when ready",
			URL:         "/api/v0/orchestrator/clickhouse/networks.csv",
			ContentType: "text/csv; charset=utf-8",
			FirstLines: []string{
				"network,name,role,site,region,country,state,city,tenant,asn",
				"1.0.0.0/24,,,,,,,,Google Inc.,15169",
				"1.128.0.0/11,,,,,,,,Telstra Pty Ltd,1221",
				"2.19.4.136/30,,,,,,,,\"Akamai Technologies, Inc.\",32787",
				"2.19.4.140/32,,,,,,,,\"Akamai Technologies, Inc.\",32787",
				"2.125.160.216/29,,,,,GB,,,,32787",
				"12.81.92.0/22,,,,,,,,AT&T Services,7018",
				"12.81.96.0/19,,,,,,,,,7018",
				"12.81.128.0/17,,,,,,,,,7018",
				"12.82.0.0/15,,,,,,,,,7018",
				"12.84.0.0/14,,,,,,,,,7018",
				"12.88.0.0/13,,,,,,,,,7018",
				"12.96.0.0/20,,,,,,,,,7018",
				"12.96.16.0/24,,,,,,,,,7018",
				"15.0.0.0/8,,,,,,,,Hewlett-Packard Company,71",
				"16.0.0.0/8,,,,,,,,Hewlett-Packard Company,71",
				"18.0.0.0/8,,,,,,,,Massachusetts Institute of Technology,3",
			},
		},
	})
}
