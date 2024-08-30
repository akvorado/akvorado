// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/s3"
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
				`192.0.2.0/24,infra,,,,,,,,`,
			},
		}, {
			URL:         "/api/v0/orchestrator/clickhouse/init.sh",
			ContentType: "text/x-shellscript",
			FirstLines: []string{
				`#!/bin/sh`,
				``,
				`# Install Protobuf schema`,
				`mkdir -p /var/lib/clickhouse/format_schemas`,
				fmt.Sprintf(`echo "Install flow schema flow-%s.proto"`,
					c.d.Schema.ProtobufMessageHash()),
				fmt.Sprintf(`cat > /var/lib/clickhouse/format_schemas/flow-%s.proto <<'EOPROTO'`,
					c.d.Schema.ProtobufMessageHash()),
				"",
				`syntax = "proto3";`,
			},
		},
		{
			URL:         "/api/v0/orchestrator/clickhouse/custom_dict_none.csv",
			ContentType: "text/plain; charset=utf-8",
			StatusCode:  404,
			FirstLines: []string{
				"unable to deliver custom dict csv file none.csv",
			},
		},
		{
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

func TestCustomDictHTTPEndpoints(t *testing.T) {
	// the httptest server is the akvorado-external upstream for the custom dict http proxy
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "col_a,col_b")
		fmt.Fprintln(w, "1,2")
	}))
	defer ts.Close()

	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	config.SkipMigrations = true

	// setup schema config for custom dicts
	schemaConfig := schema.DefaultConfiguration()
	schemaConfig.CustomDictionaries = make(map[string]schema.CustomDict)
	schemaConfig.CustomDictionaries["test"] = schema.CustomDict{
		SourceType: schema.SourceHTTP,
		Source:     ts.URL,
	}
	schemaConfig.CustomDictionaries["none"] = schema.CustomDict{
		SourceType: schema.SourceHTTP,
		Source:     "http://example.invalid/none.csv",
	}
	schemaConfig.CustomDictionaries["s3_invalid_config"] = schema.CustomDict{
		SourceType: schema.SourceS3,
		S3Config:   "invalid",
	}
	schemaConfig.CustomDictionaries["s3_no_config"] = schema.CustomDict{
		SourceType: schema.SourceS3,
	}
	sch, err := schema.New(schemaConfig)
	if err != nil {
		t.Fatalf("schema.New() error:\n%+v", err)
	}

	// create s3 stuff
	s3Config := s3.DefaultConfiguration()
	s3Component, _ := s3.New(r, s3Config, s3.Dependencies{Daemon: daemon.NewMock(t)})

	// create http entry
	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
		HTTP:   httpserver.NewMock(t, r),
		Schema: sch,
		GeoIP:  geoip.NewMock(t, r, false),
		S3:     s3Component,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	cases := helpers.HTTPEndpointCases{
		{
			URL:         "/api/v0/orchestrator/clickhouse/custom_dict_none.csv",
			ContentType: "text/plain; charset=utf-8",
			StatusCode:  500,
			FirstLines: []string{
				"unable to fetch custom dict csv file http://example.invalid/none.csv",
			},
		},
		{
			URL:         "/api/v0/orchestrator/clickhouse/custom_dict_test.csv",
			ContentType: "text/csv; charset=utf-8",
			FirstLines: []string{
				`col_a,col_b`,
				`1,2`,
			},
		},
		{
			URL:         "/api/v0/orchestrator/clickhouse/custom_dict_s3_invalid_config.csv",
			ContentType: "text/plain; charset=utf-8",
			StatusCode:  500,
			FirstLines: []string{
				"unable to fetch custom dict csv file from S3",
			},
		},
		{
			URL:         "/api/v0/orchestrator/clickhouse/custom_dict_s3_no_config.csv",
			ContentType: "text/plain; charset=utf-8",
			StatusCode:  500,
			FirstLines: []string{
				"unable to fetch custom dict csv file from S3",
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
