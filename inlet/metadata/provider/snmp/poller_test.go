// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/slayercat/GoSNMPServer"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/inlet/metadata/provider"
)

func TestPoller(t *testing.T) {
	lo := netip.MustParseAddr("::ffff:127.0.0.1")
	cases := []struct {
		Description string
		Skip        string
		Config      Configuration
		ExporterIP  netip.Addr
	}{
		{
			Description: "SNMPv2",
			Config: Configuration{
				PollerRetries: 2,
				PollerTimeout: 100 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {Communities: []string{"private"}},
				}),
				Agents: map[netip.Addr]netip.Addr{
					netip.MustParseAddr("192.0.2.1"): lo,
				},
			},
		}, {
			Description: "SNMPv2 with several communities, first",
			Config: Configuration{
				PollerRetries: 2,
				PollerTimeout: 100 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {Communities: []string{"private", "private1"}},
				}),
				Agents: map[netip.Addr]netip.Addr{
					netip.MustParseAddr("192.0.2.1"): lo,
				},
			},
		}, {
			Description: "SNMPv2 with several communities, last",
			Config: Configuration{
				PollerRetries: 2,
				PollerTimeout: 100 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {Communities: []string{"private1", "private"}},
				}),
				Agents: map[netip.Addr]netip.Addr{
					netip.MustParseAddr("192.0.2.1"): lo,
				},
			},
		}, {
			Description: "SNMPv2 with agent mapping",
			Config: Configuration{
				PollerRetries: 2,
				PollerTimeout: 100 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {Communities: []string{"private"}},
				}),
				Agents: map[netip.Addr]netip.Addr{
					netip.MustParseAddr("192.0.2.1"): lo,
				},
			},
			ExporterIP: netip.MustParseAddr("::ffff:192.0.2.1"),
		}, {
			Description: "SNMPv3",
			Config: Configuration{
				PollerRetries: 2,
				PollerTimeout: 100 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {
						UserName:                 "alfred",
						AuthenticationProtocol:   AuthProtocol(gosnmp.MD5),
						AuthenticationPassphrase: "hello",
						PrivacyProtocol:          PrivProtocol(gosnmp.AES),
						PrivacyPassphrase:        "bye",
						ContextName:              "private",
					},
				}),
			},
		}, {
			Description: "SNMPv3 no priv",
			Skip:        "GoSNMPServer is broken with this configuration",
			Config: Configuration{
				PollerRetries: 2,
				PollerTimeout: 100 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {
						UserName:                 "alfred-nopriv",
						AuthenticationProtocol:   AuthProtocol(gosnmp.MD5),
						AuthenticationPassphrase: "hello",
						PrivacyProtocol:          PrivProtocol(gosnmp.NoPriv),
						ContextName:              "private",
					},
				}),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			if tc.Skip != "" {
				t.Skip(tc.Skip)
			}
			if !tc.ExporterIP.IsValid() {
				tc.ExporterIP = lo
			}
			r := reporter.NewMock(t)

			// Start a new SNMP server
			master := GoSNMPServer.MasterAgent{
				// Logger: GoSNMPServer.NewDefaultLogger(),
				SecurityConfig: GoSNMPServer.SecurityConfig{
					AuthoritativeEngineBoots: 10,
					Users: []gosnmp.UsmSecurityParameters{
						{
							UserName:                 "alfred",
							AuthenticationProtocol:   gosnmp.MD5,
							AuthenticationPassphrase: "hello",
							PrivacyProtocol:          gosnmp.AES,
							PrivacyPassphrase:        "bye",
						}, {
							UserName:                 "alfred-nopriv",
							AuthenticationProtocol:   gosnmp.MD5,
							AuthenticationPassphrase: "hello",
							PrivacyProtocol:          gosnmp.NoPriv,
						},
					},
				},
				SubAgents: []*GoSNMPServer.SubAgent{
					{
						CommunityIDs: []string{"private"},
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
								OID:  "1.3.6.1.2.1.2.2.1.2.645",
								Type: gosnmp.OctetString,
								OnGet: func() (interface{}, error) {
									return "Correct description", nil
								},
							}, {
								OID:  "1.3.6.1.2.1.31.1.1.1.1.641",
								Type: gosnmp.OctetString,
								OnGet: func() (interface{}, error) {
									return "Gi0/0/0/0", nil
								},
							}, {
								OID:  "1.3.6.1.2.1.31.1.1.1.1.642",
								Type: gosnmp.OctetString,
								OnGet: func() (interface{}, error) {
									return "Gi0/0/0/1", nil
								},
							}, {
								OID:  "1.3.6.1.2.1.31.1.1.1.1.643",
								Type: gosnmp.OctetString,
								OnGet: func() (interface{}, error) {
									return "Gi0/0/0/2", nil
								},
							}, {
								OID:  "1.3.6.1.2.1.31.1.1.1.1.645",
								Type: gosnmp.OctetString,
								OnGet: func() (interface{}, error) {
									return "Gi0/0/0/5", nil
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
								OID:  "1.3.6.1.2.1.31.1.1.1.15.645",
								Type: gosnmp.Gauge32,
								OnGet: func() (interface{}, error) {
									return uint(1000), nil
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
							{
								OID:  "1.3.6.1.2.1.31.1.1.1.18.645",
								Type: gosnmp.OctetString,
								OnGet: func() (interface{}, error) {
									return "GigabitEthernet-something", nil
								},
							},
						},
					},
				},
			}
			server := GoSNMPServer.NewSNMPServer(master)
			if err := server.ListenUDP("udp", "127.0.0.1:0"); err != nil {
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
			r.Debug().Int("port", port).Msg("SNMP server listening")
			go server.ServeForever()
			defer server.Shutdown()

			got := []string{}
			config := tc.Config
			config.Ports = helpers.MustNewSubnetMap(map[string]uint16{
				"::/0": uint16(port),
			})
			put := func(update provider.Update) {
				got = append(got, fmt.Sprintf("%s %s %d %s %s %d",
					update.ExporterIP.Unmap().String(), update.Exporter.Name,
					update.IfIndex, update.Interface.Name, update.Interface.Description, update.Interface.Speed))
			}
			p, err := config.New(r, put)
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}

			p.Query(context.Background(), provider.BatchQuery{ExporterIP: tc.ExporterIP, IfIndexes: []uint{641}})
			p.Query(context.Background(), provider.BatchQuery{ExporterIP: tc.ExporterIP, IfIndexes: []uint{642}})
			p.Query(context.Background(), provider.BatchQuery{ExporterIP: tc.ExporterIP, IfIndexes: []uint{643, 644, 645}})
			p.Query(context.Background(), provider.BatchQuery{ExporterIP: tc.ExporterIP, IfIndexes: []uint{0}})
			exporterStr := tc.ExporterIP.Unmap().String()
			time.Sleep(50 * time.Millisecond)
			if diff := helpers.Diff(got, []string{
				fmt.Sprintf(`%s exporter62 641 Gi0/0/0/0 Transit 10000`, exporterStr),
				fmt.Sprintf(`%s exporter62 642 Gi0/0/0/1 Peering 20000`, exporterStr),
				fmt.Sprintf(`%s exporter62 643 Gi0/0/0/2  10000`, exporterStr),                   // no ifAlias
				fmt.Sprintf(`%s exporter62 644   0`, exporterStr),                                // negative cache
				fmt.Sprintf(`%s exporter62 645 Gi0/0/0/5 Correct description 1000`, exporterStr), // negative cache
				fmt.Sprintf(`%s exporter62 0   0`, exporterStr),
			}); diff != "" {
				t.Fatalf("Poll() (-got, +want):\n%s", diff)
			}

			gotMetrics := r.GetMetrics("akvorado_inlet_metadata_provider_snmp_poller_", "error_", "pending_", "success_")
			expectedMetrics := map[string]string{
				fmt.Sprintf(`error_requests_total{error="ifalias missing",exporter="%s"}`, exporterStr): "2", // 643+644
				fmt.Sprintf(`error_requests_total{error="ifdescr missing",exporter="%s"}`, exporterStr): "1", // 644
				fmt.Sprintf(`error_requests_total{error="ifname missing",exporter="%s"}`, exporterStr):  "1", // 644
				fmt.Sprintf(`error_requests_total{error="ifspeed missing",exporter="%s"}`, exporterStr): "1", // 644
				`pending_requests`: "0",
				fmt.Sprintf(`success_requests_total{exporter="%s"}`, exporterStr): "5", // 641+642+643+645+0
			}
			if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
				t.Fatalf("Metrics (-got, +want):\n%s", diff)
			}
		})
	}
}
