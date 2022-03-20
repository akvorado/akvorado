package clickhouse

import (
	"testing"

	"akvorado/helpers"
	"akvorado/http"
	"akvorado/reporter"
)

func TestHTTPEndpoints(t *testing.T) {
	r := reporter.NewMock(t)
	c, err := New(r, DefaultConfiguration, Dependencies{
		HTTP: http.NewMock(t, r),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	cases := helpers.HTTPEndpointCases{
		{
			URL:         "/api/v0/clickhouse/protocols.csv",
			ContentType: "text/csv; charset=utf-8",
			FirstLines: []string{
				`proto,name,description`,
				`0,HOPOPT,IPv6 Hop-by-Hop Option`,
				`1,ICMP,Internet Control Message`,
			},
		}, {
			URL:         "/api/v0/clickhouse/asns.csv",
			ContentType: "text/csv; charset=utf-8",
			FirstLines: []string{
				"asn,name",
				"1,LVLT-1",
			},
		}, {
			URL:         "/api/v0/clickhouse/init.sh",
			ContentType: "text/x-shellscript",
			FirstLines: []string{
				`#!/bin/sh`,
				``,
				`cat > /var/lib/clickhouse/format_schemas/flow-0.proto <<'EOPROTO'`,
				`syntax = "proto3";`,
				`package flow;`,
			},
		},
	}

	helpers.TestHTTPEndpoints(t, c.d.HTTP.Address, cases)
}
