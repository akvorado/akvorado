// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

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
	config := DefaultConfiguration()
	config.Networks = NetworkNames{
		"::ffff:192.0.2.0/24": "infra",
	}
	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
		HTTP:   http.NewMock(t, r),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

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
				`network,name`,
				`::ffff:192.0.2.0/24,infra`,
			},
		}, {
			URL:         "/api/v0/orchestrator/clickhouse/init.sh",
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

func TestAdditionalASNs(t *testing.T) {
	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	config.ASNs = map[uint32]string{
		1: "New network",
	}
	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
		HTTP:   http.NewMock(t, r),
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

	helpers.TestHTTPEndpoints(t, c.d.HTTP.Address, cases)
}
