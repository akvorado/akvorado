// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/slayercat/GoSNMPServer"
	"github.com/slayercat/gosnmp"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestPoller(t *testing.T) {
	got := []string{}
	r := reporter.NewMock(t)
	clock := clock.NewMock()
	config := pollerConfig{
		Retries: 2,
		Timeout: 100 * time.Millisecond,
	}
	p := newPoller(r, config, clock, func(exporterIP, exporterName string, ifIndex uint, iface Interface) {
		got = append(got, fmt.Sprintf("%s %s %d %s %s %d", exporterIP, exporterName,
			ifIndex, iface.Name, iface.Description, iface.Speed))
	})

	// Start a new SNMP server
	master := GoSNMPServer.MasterAgent{
		SubAgents: []*GoSNMPServer.SubAgent{
			{
				CommunityIDs: []string{"public"},
				OIDs: []*GoSNMPServer.PDUValueControlItem{
					{
						OID:  "1.3.6.1.2.1.1.5.0",
						Type: gosnmp.OctetString,
						OnGet: func() (interface{}, error) {
							return "exporter62", nil
						},
					}, {
						OID:  "1.3.6.1.2.1.2.2.1.2.641",
						Type: gosnmp.OctetString,
						OnGet: func() (interface{}, error) {
							return "Gi0/0/0/0", nil
						},
					}, {
						OID:  "1.3.6.1.2.1.2.2.1.2.642",
						Type: gosnmp.OctetString,
						OnGet: func() (interface{}, error) {
							return "Gi0/0/0/1", nil
						},
					}, {
						OID:  "1.3.6.1.2.1.2.2.1.2.643",
						Type: gosnmp.OctetString,
						OnGet: func() (interface{}, error) {
							return "Gi0/0/0/2", nil
						},
					}, {
						OID:  "1.3.6.1.2.1.31.1.1.1.15.641",
						Type: gosnmp.Gauge32,
						OnGet: func() (interface{}, error) {
							return uint(10000), nil
						},
					}, {
						OID:  "1.3.6.1.2.1.31.1.1.1.15.642",
						Type: gosnmp.Gauge32,
						OnGet: func() (interface{}, error) {
							return uint(20000), nil
						},
					}, {
						OID:  "1.3.6.1.2.1.31.1.1.1.15.643",
						Type: gosnmp.Gauge32,
						OnGet: func() (interface{}, error) {
							return uint(10000), nil
						},
					}, {
						OID:  "1.3.6.1.2.1.31.1.1.1.18.641",
						Type: gosnmp.OctetString,
						OnGet: func() (interface{}, error) {
							return "Transit", nil
						},
					}, {
						OID:  "1.3.6.1.2.1.31.1.1.1.18.642",
						Type: gosnmp.OctetString,
						OnGet: func() (interface{}, error) {
							return "Peering", nil
						},
					},
					// ifAlias.643 missing
				},
			},
		},
	}
	server := GoSNMPServer.NewSNMPServer(master)
	err := server.ListenUDP("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenUDP() err:\n%+v", err)
	}
	_, portStr, err := net.SplitHostPort(server.Address().String())
	if err != nil {
		panic(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(err)
	}
	p.r.Debug().Int("port", port).Msg("SNMP server listening")
	go server.ServeForever()
	defer server.Shutdown()

	p.Poll(context.Background(), "127.0.0.1", uint16(port), "public", []uint{641})
	p.Poll(context.Background(), "127.0.0.1", uint16(port), "public", []uint{642})
	p.Poll(context.Background(), "127.0.0.1", uint16(port), "public", []uint{643})
	p.Poll(context.Background(), "127.0.0.1", uint16(port), "public", []uint{644})
	p.Poll(context.Background(), "127.0.0.1", uint16(port), "public", []uint{0})
	time.Sleep(50 * time.Millisecond)
	if diff := helpers.Diff(got, []string{
		`127.0.0.1 exporter62 641 Gi0/0/0/0 Transit 10000`,
		`127.0.0.1 exporter62 642 Gi0/0/0/1 Peering 20000`,
		`127.0.0.1 exporter62 0 unknown  0`,
	}); diff != "" {
		t.Fatalf("Poll() (-got, +want):\n%s", diff)
	}

	gotMetrics := r.GetMetrics("akvorado_inlet_snmp_poller_", "failure_", "pending_", "success_")
	expectedMetrics := map[string]string{
		`failure_requests{error="ifalias missing",exporter="127.0.0.1"}`: "2", // 643+644
		`failure_requests{error="ifdescr missing",exporter="127.0.0.1"}`: "1", // 644
		`failure_requests{error="ifspeed missing",exporter="127.0.0.1"}`: "1", // 644
		`pending_requests`:                       "0",
		`success_requests{exporter="127.0.0.1"}`: "3", // 641+642+0
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

}
