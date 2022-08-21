// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp_test

import (
	"net"
	"net/netip"
	"testing"
	"time"

	gobmp "github.com/osrg/gobgp/v3/pkg/packet/bmp"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/demoexporter/bmp"
)

func TestClient(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error:\n%+v", err)
	}
	defer listener.Close()

	config := bmp.DefaultConfiguration()
	config.Target = listener.Addr().String()
	config.RetryAfter = 0
	config.StatsDelay = 10 * time.Millisecond
	config.Routes = []bmp.RouteConfiguration{
		{
			Prefixes:    []netip.Prefix{netip.MustParsePrefix("2001:db8::/64")},
			ASPath:      []uint32{65001, 65002, 65002},
			Communities: []bmp.Community{500, 600, 700},
		}, {
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("192.0.2.0/24"),
				netip.MustParsePrefix("203.0.113.0/24"),
			},
			ASPath: []uint32{12322, 1299},
		}, {
			Prefixes: []netip.Prefix{
				netip.MustParsePrefix("192.0.2.0/24"),
				netip.MustParsePrefix("2001:db8::/64"),
			},
			ASPath: []uint32{65001, 65002},
		},
	}
	r := reporter.NewMock(t)
	c, err := bmp.New(r, config, bmp.Dependencies{
		Daemon: daemon.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	// Test we get a reconnect
	conn, err := listener.Accept()
	time.Sleep(20 * time.Millisecond)
	conn.Close()
	conn, err = listener.Accept()
	if err != nil {
		t.Fatalf("Accept() error:\n%+v", err)
	}
	defer conn.Close()

	got := make([]byte, 5000)
	n, err := conn.Read(got)
	if err != nil {
		t.Fatalf("Read() error:\n%+v", err)
	}
	got = got[:n]

	msgs := []*gobmp.BMPMessage{}
	for {
		advance, token, err := gobmp.SplitBMP(got, len(got) > 0)
		if err != nil {
			t.Fatalf("SplitBMP() error:\n%+v", err)
		}
		if token == nil {
			break
		}
		t.Logf("BMP message len: %d", len(token))
		msg, err := gobmp.ParseBMPMessage(token)
		if err != nil {
			t.Fatalf("ParseBMPMessage() error:\n%+v", err)
		}
		msgs = append(msgs, msg)
		got = got[advance:]
	}

	// Assume we got what we want.

	time.Sleep(20 * time.Millisecond)
	gotMetrics := r.GetMetrics("akvorado_demoexporter_")
	expectedMetrics := map[string]string{
		`bmp_connections_total`:         "2",
		`bmp_errors_total{error="EOF"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}
