package clickhouse

import (
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestHTTPEndpoints(t *testing.T) {
	r := reporter.NewMock(t)
	c, err := New(r, DefaultConfiguration, Dependencies{
		Daemon: daemon.NewMock(t),
		HTTP:   http.NewMock(t, r),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	cases := helpers.HTTPEndpointCases{
		{
			URL:         "/api/v0/configure/clickhouse/protocols.csv",
			ContentType: "text/csv; charset=utf-8",
			FirstLines: []string{
				`proto,name,description`,
				`0,HOPOPT,IPv6 Hop-by-Hop Option`,
				`1,ICMP,Internet Control Message`,
			},
		}, {
			URL:         "/api/v0/configure/clickhouse/asns.csv",
			ContentType: "text/csv; charset=utf-8",
			FirstLines: []string{
				"asn,name",
				"1,LVLT-1",
			},
		}, {
			URL:         "/api/v0/configure/clickhouse/init.sh",
			ContentType: "text/x-shellscript",
			FirstLines: []string{
				`#!/bin/sh`,
				``,
				`cat > /var/lib/clickhouse/format_schemas/flow-0.proto <<'EOPROTO'`,
				`syntax = "proto3";`,
				`package decoder;`,
			},
		},
	}

	helpers.TestHTTPEndpoints(t, c.d.HTTP.Address, cases)
}
