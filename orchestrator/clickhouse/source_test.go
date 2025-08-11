// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"akvorado/common/clickhousedb"
	"akvorado/common/remotedatasource"
	"akvorado/orchestrator/geoip"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

func TestNetworkSources(t *testing.T) {
	r := reporter.NewMock(t)
	clickhouseComponent := clickhousedb.SetupClickHouse(t, r, false)

	// Mux to answer requests
	ready := make(chan bool)
	mux := http.NewServeMux()
	mux.Handle("/amazon.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	server := &http.Server{
		Addr:    listener.Addr().String(),
		Handler: mux,
	}
	address := listener.Addr()
	go server.Serve(listener)
	defer server.Shutdown(context.Background())

	config := DefaultConfiguration()
	config.SkipMigrations = true
	config.NetworkSourcesTimeout = 10 * time.Millisecond
	config.NetworkSources = map[string]remotedatasource.Source{
		"amazon": {
			URL:    fmt.Sprintf("http://%s/amazon.json", address),
			Method: "GET",
			Headers: map[string]string{
				"X-Foo": "hello",
			},
			Timeout:  20 * time.Millisecond,
			Interval: 100 * time.Millisecond,
			Transform: remotedatasource.MustParseTransformQuery(`
(.prefixes + .ipv6_prefixes)[] |
{ prefix: (.ip_prefix // .ipv6_prefix), tenant: "amazon", region: .region, role: .service|ascii_downcase }
`),
		},
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
				`network,name,role,site,region,country,state,city,tenant,asn`,
				`::ffff:3.2.34.0/122,,amazon,,af-south-1,,,,amazon,`,
				`2600:1f14:fff:f800::/56,,route53_healthchecks,,us-west-2,,,,amazon,`,
				`2600:1ff2:4000::/40,,amazon,,us-west-2,,,,amazon,`,
			},
		},
	})

	gotMetrics := r.GetMetrics("akvorado_common_remotedatasource_data_")
	expectedMetrics := map[string]string{
		`total{source="amazon",type="network_source"}`: "3",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}
