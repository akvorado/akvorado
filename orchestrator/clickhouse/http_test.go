// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"fmt"
	"net"
	netHTTP "net/http"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestHTTPEndpoints(t *testing.T) {
	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	config.SkipMigrations = true
	config.Networks = helpers.MustNewSubnetMap(map[string]NetworkAttributes{
		"::ffff:192.0.2.0/120": {Name: "infra"},
	})
	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
		HTTP:   http.NewMock(t, r),
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
				`network,name,role,site,region,tenant`,
				`192.0.2.0/24,infra,,,,`,
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

	helpers.TestHTTPEndpoints(t, c.d.HTTP.LocalAddr(), cases)
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

	helpers.TestHTTPEndpoints(t, c.d.HTTP.LocalAddr(), cases)
}

func TestNetworkSources(t *testing.T) {
	// Mux to answer requests
	ready := make(chan bool)
	mux := netHTTP.NewServeMux()
	mux.Handle("/amazon.json", netHTTP.HandlerFunc(func(w netHTTP.ResponseWriter, r *netHTTP.Request) {
		select {
		case <-ready:
		default:
			w.WriteHeader(404)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`
{
  "syncToken": "1665609189",
  "createDate": "2022-10-12-21-13-09",
  "prefixes": [
    {
      "ip_prefix": "3.2.34.0/26",
      "region": "af-south-1",
      "service": "AMAZON",
      "network_border_group": "af-south-1"
    }
  ],
  "ipv6_prefixes": [
    {
      "ipv6_prefix": "2600:1ff2:4000::/40",
      "region": "us-west-2",
      "service": "AMAZON",
      "network_border_group": "us-west-2"
    },
    {
      "ipv6_prefix": "2600:1f14:fff:f800::/56",
      "region": "us-west-2",
      "service": "ROUTE53_HEALTHCHECKS",
      "network_border_group": "us-west-2"
    }
  ]
}
`))
	}))

	// Setup an HTTP server to serve the JSON
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error:\n%+v", err)
	}
	server := &netHTTP.Server{
		Addr:    listener.Addr().String(),
		Handler: mux,
	}
	address := listener.Addr()
	go server.Serve(listener)
	defer server.Shutdown(context.Background())

	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	config.SkipMigrations = true
	config.NetworkSourcesTimeout = 10 * time.Millisecond
	config.NetworkSources = map[string]NetworkSource{
		"amazon": {
			URL:      fmt.Sprintf("http://%s/amazon.json", address),
			Interval: 100 * time.Millisecond,
			Transform: MustParseTransformQuery(`
(.prefixes + .ipv6_prefixes)[] |
{ prefix: (.ip_prefix // .ipv6_prefix), tenant: "amazon", region: .region, role: .service|ascii_downcase }
`),
		},
	}
	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
		HTTP:   http.NewMock(t, r),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	// When not ready, we get a 503
	helpers.TestHTTPEndpoints(t, c.d.HTTP.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "try when not ready",
			URL:         "/api/v0/orchestrator/clickhouse/networks.csv",
			StatusCode:  503,
		},
	})
	close(ready)
	time.Sleep(50 * time.Millisecond)
	helpers.TestHTTPEndpoints(t, c.d.HTTP.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "try when ready",
			URL:         "/api/v0/orchestrator/clickhouse/networks.csv",
			ContentType: "text/csv; charset=utf-8",
			FirstLines: []string{
				`network,name,role,site,region,tenant`,
				`3.2.34.0/26,,amazon,,af-south-1,amazon`,
				`2600:1ff2:4000::/40,,amazon,,us-west-2,amazon`,
				`2600:1f14:fff:f800::/56,,route53_healthchecks,,us-west-2,amazon`,
			},
		},
	})

	gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_network_source_networks_")
	expectedMetrics := map[string]string{
		`total{source="amazon"}`: "3",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

}
