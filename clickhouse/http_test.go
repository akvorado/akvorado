package clickhouse

import (
	"bufio"
	"fmt"
	netHTTP "net/http"
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

	cases := []struct {
		URL         string
		ContentType string
		FirstLines  []string
	}{
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
			URL:         "/api/v0/clickhouse/flow.proto",
			ContentType: "text/plain",
			FirstLines: []string{
				`syntax = "proto3";`,
				`package flow;`,
			},
		}, {
			URL:         "/api/v0/clickhouse/init.sh",
			ContentType: "text/x-shellscript",
			FirstLines: []string{
				`#!/bin/sh`,
				`cat > /var/lib/clickhouse/format_schemas/flow.proto <<'EOF'`,
				`syntax = "proto3";`,
				`package flow;`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.URL, func(t *testing.T) {
			resp, err := netHTTP.Get(fmt.Sprintf("http://%s%s", c.d.HTTP.Address, tc.URL))
			if err != nil {
				t.Fatalf("GET %s:\n%+v", tc.URL, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("GET %s: got status code %d, not 200", tc.URL, resp.StatusCode)
			}
			gotContentType := resp.Header.Get("Content-Type")
			if gotContentType != tc.ContentType {
				t.Errorf("GET %s Content-Type (-got, +want):\n-%s\n+%s",
					tc.URL, gotContentType, tc.ContentType)
			}
			reader := bufio.NewScanner(resp.Body)
			got := []string{}
			for reader.Scan() && len(got) < len(tc.FirstLines) {
				got = append(got, reader.Text())
			}
			if diff := helpers.Diff(got, tc.FirstLines); diff != "" {
				t.Errorf("GET %s (-got, +want):\n%s", tc.URL, diff)
			}
		})
	}

}
