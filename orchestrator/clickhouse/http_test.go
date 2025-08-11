// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"testing"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/orchestrator/geoip"
)

func TestHTTPEndpoints(t *testing.T) {
	r := reporter.NewMock(t)
	clickhouseComponent := clickhousedb.SetupClickHouse(t, r, false)
	config := DefaultConfiguration()
	config.SkipMigrations = true
	config.Networks = helpers.MustNewSubnetMap(map[string]NetworkAttributes{
		"::ffff:192.0.2.0/120": {Name: "infra"},
	})
	// setup schema config for custom dicts
	schemaConfig := schema.DefaultConfiguration()
	schemaConfig.CustomDictionaries = make(map[string]schema.CustomDict)
	schemaConfig.CustomDictionaries["test"] = schema.CustomDict{
		Source: "testdata/dicts/test.csv",
	}
	schemaConfig.CustomDictionaries["none"] = schema.CustomDict{
		Source: "none.csv",
	}

	sch, err := schema.New(schemaConfig)
	if err != nil {
		t.Fatalf("schema.New() error:\n%+v", err)
	}
	// create http entry
	c, err := New(r, config, Dependencies{
		Daemon:     daemon.NewMock(t),
		HTTP:       httpserver.NewMock(t, r),
		Schema:     sch,
		GeoIP:      geoip.NewMock(t, r, false),
		ClickHouse: clickhouseComponent,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	cases := helpers.HTTPEndpointCases{
		{
			URL:         "/api/v0/orchestrator/clickhouse/protocols.csv",
			ContentType: "text/csv; charset=utf-8",
			FirstLines: []string{
				`proto,name,description`,
				`0,HOPOPT,IPv6 Hop-by-Hop Option`,
				`1,ICMP,Internet Control Message`,
			},
		}, {
			URL:         "/api/v0/orchestrator/clickhouse/asns.csv",
			ContentType: "text/csv; charset=utf-8",
			FirstLines: []string{
				`"asn","name"`,
				`1,"Level 3 Communications"`,
			},
		}, {
			URL:         "/api/v0/orchestrator/clickhouse/networks.csv",
			ContentType: "text/csv; charset=utf-8",
			FirstLines: []string{
				`network,name,role,site,region,country,state,city,tenant,asn`,
				`::ffff:192.0.2.0/120,infra,,,,,,,,`,
			},
		}, {
			URL:         "/api/v0/orchestrator/clickhouse/custom_dict_none.csv",
			ContentType: "text/plain; charset=utf-8",
			StatusCode:  404,
			FirstLines: []string{
				"unable to deliver custom dict csv file none.csv",
			},
		}, {
			URL:         "/api/v0/orchestrator/clickhouse/custom_dict_test.csv",
			ContentType: "text/csv; charset=utf-8",
			FirstLines: []string{
				`col_a,col_b`,
				`1,2`,
			},
		},
	}

	helpers.TestHTTPEndpoints(t, c.d.HTTP.LocalAddr(), cases)
}

func TestAdditionalASNs(t *testing.T) {
	r := reporter.NewMock(t)
	clickhouseComponent := clickhousedb.SetupClickHouse(t, r, false)
	config := DefaultConfiguration()
	config.ASNs = map[uint32]string{
		1: "New network",
	}
	c, err := New(r, config, Dependencies{
		Daemon:     daemon.NewMock(t),
		HTTP:       httpserver.NewMock(t, r),
		Schema:     schema.NewMock(t),
		GeoIP:      geoip.NewMock(t, r, false),
		ClickHouse: clickhouseComponent,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	cases := helpers.HTTPEndpointCases{
		{
			URL:         "/api/v0/orchestrator/clickhouse/asns.csv",
			ContentType: "text/csv; charset=utf-8",
			FirstLines: []string{
				`asn,name`,
				`1,New network`,
				`2,University of Delaware`,
			},
		},
	}

	helpers.TestHTTPEndpoints(t, c.d.HTTP.LocalAddr(), cases)
}
