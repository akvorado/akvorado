// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/proto"
	"github.com/mitchellh/mapstructure"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
	"akvorado/inlet/flow"
	"akvorado/inlet/geoip"
	"akvorado/inlet/kafka"
	"akvorado/inlet/snmp"
)

func TestHydrate(t *testing.T) {
	cases := []struct {
		Name          string
		Configuration gin.H
		InputFlow     func() *flow.Message
		OutputFlow    *flow.Message
	}{
		{
			Name:          "no rule",
			Configuration: gin.H{},
			InputFlow: func() *flow.Message {
				return &flow.Message{
					SamplingRate:    1000,
					ExporterAddress: net.ParseIP("192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &flow.Message{
				SamplingRate:     1000,
				ExporterAddress:  net.ParseIP("192.0.2.142"),
				ExporterName:     "192_0_2_142",
				InIf:             100,
				OutIf:            200,
				InIfName:         "Gi0/0/100",
				OutIfName:        "Gi0/0/200",
				InIfDescription:  "Interface 100",
				OutIfDescription: "Interface 200",
				InIfSpeed:        1000,
				OutIfSpeed:       1000,
			},
		},
		{
			Name: "no rule, override sampling rate",
			Configuration: gin.H{"overridesamplingrate": gin.H{
				"192.0.2.0/24":   100,
				"192.0.2.128/25": 500,
				"192.0.2.141/32": 1000,
			}},
			InputFlow: func() *flow.Message {
				return &flow.Message{
					SamplingRate:    1000,
					ExporterAddress: net.ParseIP("192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &flow.Message{
				SamplingRate:     500,
				ExporterAddress:  net.ParseIP("192.0.2.142"),
				ExporterName:     "192_0_2_142",
				InIf:             100,
				OutIf:            200,
				InIfName:         "Gi0/0/100",
				OutIfName:        "Gi0/0/200",
				InIfDescription:  "Interface 100",
				OutIfDescription: "Interface 200",
				InIfSpeed:        1000,
				OutIfSpeed:       1000,
			},
		},
		{
			Name:          "no rule, no sampling rate, default is one value",
			Configuration: gin.H{"defaultsamplingrate": 500},
			InputFlow: func() *flow.Message {
				return &flow.Message{
					ExporterAddress: net.ParseIP("192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &flow.Message{
				SamplingRate:     500,
				ExporterAddress:  net.ParseIP("192.0.2.142"),
				ExporterName:     "192_0_2_142",
				InIf:             100,
				OutIf:            200,
				InIfName:         "Gi0/0/100",
				OutIfName:        "Gi0/0/200",
				InIfDescription:  "Interface 100",
				OutIfDescription: "Interface 200",
				InIfSpeed:        1000,
				OutIfSpeed:       1000,
			},
		},
		{
			Name: "no rule, no sampling rate, default is map",
			Configuration: gin.H{"defaultsamplingrate": gin.H{
				"192.0.2.0/24":   100,
				"192.0.2.128/25": 500,
				"192.0.2.141/32": 1000,
			}},
			InputFlow: func() *flow.Message {
				return &flow.Message{
					ExporterAddress: net.ParseIP("192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &flow.Message{
				SamplingRate:     500,
				ExporterAddress:  net.ParseIP("192.0.2.142"),
				ExporterName:     "192_0_2_142",
				InIf:             100,
				OutIf:            200,
				InIfName:         "Gi0/0/100",
				OutIfName:        "Gi0/0/200",
				InIfDescription:  "Interface 100",
				OutIfDescription: "Interface 200",
				InIfSpeed:        1000,
				OutIfSpeed:       1000,
			},
		},
		{
			Name: "exporter rule",
			Configuration: gin.H{
				"exporterclassifiers": []string{
					`Exporter.Name startsWith "hello" && ClassifyRegion("europe")`,
					`Exporter.Name startsWith "192_" && ClassifyRegion("asia")`,
					`ClassifyRegion("other") && ClassifySite("unknown") && ClassifyTenant("alfred")`,
				},
			},
			InputFlow: func() *flow.Message {
				return &flow.Message{
					SamplingRate:    1000,
					ExporterAddress: net.ParseIP("192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &flow.Message{
				SamplingRate:     1000,
				ExporterAddress:  net.ParseIP("192.0.2.142"),
				ExporterName:     "192_0_2_142",
				ExporterRegion:   "asia",
				ExporterTenant:   "alfred",
				ExporterSite:     "unknown",
				InIf:             100,
				OutIf:            200,
				InIfName:         "Gi0/0/100",
				OutIfName:        "Gi0/0/200",
				InIfDescription:  "Interface 100",
				OutIfDescription: "Interface 200",
				InIfSpeed:        1000,
				OutIfSpeed:       1000,
			},
		},
		{
			Name: "interface rule",
			Configuration: gin.H{
				"interfaceclassifiers": []string{
					`
Interface.Description startsWith "Transit:" &&
ClassifyConnectivity("transit") &&
ClassifyExternal() &&
ClassifyProviderRegex(Interface.Description, "^Transit: ([^ ]+)", "$1")`,
					`ClassifyInternal()`,
				},
			},
			InputFlow: func() *flow.Message {
				return &flow.Message{
					SamplingRate:    1000,
					ExporterAddress: net.ParseIP("192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &flow.Message{
				SamplingRate:     1000,
				ExporterAddress:  net.ParseIP("192.0.2.142"),
				ExporterName:     "192_0_2_142",
				InIf:             100,
				OutIf:            200,
				InIfName:         "Gi0/0/100",
				OutIfName:        "Gi0/0/200",
				InIfDescription:  "Interface 100",
				OutIfDescription: "Interface 200",
				InIfSpeed:        1000,
				OutIfSpeed:       1000,
				InIfBoundary:     2, // Internal
				OutIfBoundary:    2,
			},
		},
		{
			Name: "configure twice boundary",
			Configuration: gin.H{
				"interfaceclassifiers": []string{
					`ClassifyInternal()`,
					`ClassifyExternal()`,
				},
			},
			InputFlow: func() *flow.Message {
				return &flow.Message{
					SamplingRate:    1000,
					ExporterAddress: net.ParseIP("192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &flow.Message{
				SamplingRate:     1000,
				ExporterAddress:  net.ParseIP("192.0.2.142"),
				ExporterName:     "192_0_2_142",
				InIf:             100,
				OutIf:            200,
				InIfName:         "Gi0/0/100",
				OutIfName:        "Gi0/0/200",
				InIfDescription:  "Interface 100",
				OutIfDescription: "Interface 200",
				InIfSpeed:        1000,
				OutIfSpeed:       1000,
				InIfBoundary:     2, // Internal
				OutIfBoundary:    2,
			},
		},
		{
			Name: "configure twice provider",
			Configuration: gin.H{
				"interfaceclassifiers": []string{
					`ClassifyProvider("telia")`,
					`ClassifyProvider("cogent")`,
				},
			},
			InputFlow: func() *flow.Message {
				return &flow.Message{
					SamplingRate:    1000,
					ExporterAddress: net.ParseIP("192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &flow.Message{
				SamplingRate:     1000,
				ExporterAddress:  net.ParseIP("192.0.2.142"),
				ExporterName:     "192_0_2_142",
				InIf:             100,
				OutIf:            200,
				InIfName:         "Gi0/0/100",
				OutIfName:        "Gi0/0/200",
				InIfDescription:  "Interface 100",
				OutIfDescription: "Interface 200",
				InIfSpeed:        1000,
				OutIfSpeed:       1000,
				InIfProvider:     "telia",
				OutIfProvider:    "telia",
			},
		},
		{
			Name: "classify depending on description",
			Configuration: gin.H{
				"interfaceclassifiers": []string{
					`ClassifyProvider("Othello")`,
					`ClassifyConnectivityRegex(Interface.Description, " (1\\d+)$", "P$1") && ClassifyExternal()`,
					`ClassifyInternal() && ClassifyConnectivity("core")`,
				},
			},
			InputFlow: func() *flow.Message {
				return &flow.Message{
					SamplingRate:    1000,
					ExporterAddress: net.ParseIP("192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &flow.Message{
				SamplingRate:      1000,
				ExporterAddress:   net.ParseIP("192.0.2.142"),
				ExporterName:      "192_0_2_142",
				InIf:              100,
				OutIf:             200,
				InIfName:          "Gi0/0/100",
				OutIfName:         "Gi0/0/200",
				InIfDescription:   "Interface 100",
				OutIfDescription:  "Interface 200",
				InIfSpeed:         1000,
				OutIfSpeed:        1000,
				InIfConnectivity:  "p100",
				OutIfConnectivity: "core",
				InIfProvider:      "othello",
				OutIfProvider:     "othello",
				InIfBoundary:      1, // external
				OutIfBoundary:     2, // internal
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			r := reporter.NewMock(t)

			// Prepare all components.
			daemonComponent := daemon.NewMock(t)
			snmpComponent := snmp.NewMock(t, r, snmp.DefaultConfiguration(),
				snmp.Dependencies{Daemon: daemonComponent})
			flowComponent := flow.NewMock(t, r, flow.DefaultConfiguration())
			geoipComponent := geoip.NewMock(t, r)
			kafkaComponent, kafkaProducer := kafka.NewMock(t, r, kafka.DefaultConfiguration())
			httpComponent := http.NewMock(t, r)

			// Prepare a configuration
			configuration := DefaultConfiguration()
			decoder, err := mapstructure.NewDecoder(helpers.GetMapStructureDecoderConfig(&configuration))
			if err != nil {
				t.Fatalf("NewDecoder() error:\n%+v", err)
			}
			if err := decoder.Decode(tc.Configuration); err != nil {
				t.Fatalf("Decode() error:\n%+v", err)
			}

			// Instantiate and start core
			c, err := New(r, configuration, Dependencies{
				Daemon: daemonComponent,
				Flow:   flowComponent,
				Snmp:   snmpComponent,
				GeoIP:  geoipComponent,
				Kafka:  kafkaComponent,
				HTTP:   httpComponent,
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, c)

			// Inject twice since otherwise, we get a cache miss
			received := make(chan bool)
			kafkaProducer.ExpectInputWithMessageCheckerFunctionAndSucceed(
				func(msg *sarama.ProducerMessage) error {
					defer close(received)
					got := flow.Message{}
					b, err := msg.Value.Encode()
					if err != nil {
						t.Fatalf("Kafka message encoding error:\n%+v", err)
					}
					buf := proto.NewBuffer(b)
					err = buf.DecodeMessage(&got)
					if err != nil {
						t.Fatalf("Kakfa message decode error:\n%+v", err)
					}

					if diff := helpers.Diff(&got, tc.OutputFlow); diff != "" {
						t.Errorf("Classifier (-got, +want):\n%s", diff)
					}
					return nil
				})

			flowComponent.Inject(t, tc.InputFlow())
			time.Sleep(50 * time.Millisecond) // Needed to let poller does its job
			flowComponent.Inject(t, tc.InputFlow())
			select {
			case <-received:
			case <-time.After(1 * time.Second):
				t.Fatal("Kafka message not received")
			}
			gotMetrics := r.GetMetrics("akvorado_inlet_core_flows_")
			expectedMetrics := map[string]string{
				`errors{error="SNMP cache miss",exporter="192.0.2.142"}`: "1",
				`http_clients`:                      "0",
				`received{exporter="192.0.2.142"}`:  "2",
				`forwarded{exporter="192.0.2.142"}`: "1",
			}
			if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
				t.Fatalf("Metrics (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestGetASNumber(t *testing.T) {
	cases := []struct {
		Flow      uint32
		Addr      string
		Providers []ASNProvider
		Expected  uint32
	}{
		{12322, "1.0.0.1", []ASNProvider{ProviderFlow}, 12322},
		{65536, "1.0.0.1", []ASNProvider{ProviderFlow}, 65536},
		{65536, "1.0.0.1", []ASNProvider{ProviderFlowExceptPrivate}, 0},
		{4_200_000_121, "1.0.0.1", []ASNProvider{ProviderFlowExceptPrivate}, 0},
		{65536, "1.0.0.1", []ASNProvider{ProviderFlowExceptPrivate, ProviderFlow}, 65536},
		{12322, "1.0.0.1", []ASNProvider{ProviderFlowExceptPrivate}, 12322},
		{12322, "1.0.0.1", []ASNProvider{ProviderGeoIP}, 15169},
		{12322, "2.0.0.1", []ASNProvider{ProviderGeoIP}, 0},
		{12322, "1.0.0.1", []ASNProvider{ProviderGeoIP, ProviderFlow}, 15169},
		{12322, "1.0.0.1", []ASNProvider{ProviderFlow, ProviderGeoIP}, 12322},
		{12322, "2.0.0.1", []ASNProvider{ProviderFlow, ProviderGeoIP}, 12322},
		{12322, "2.0.0.1", []ASNProvider{ProviderGeoIP, ProviderFlow}, 12322},
	}
	for i, tc := range cases {
		i++
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			r := reporter.NewMock(t)

			// We don't need all components as we won't start the component.
			configuration := DefaultConfiguration()
			configuration.ASNProviders = tc.Providers
			c, err := New(r, configuration, Dependencies{
				Daemon: daemon.NewMock(t),
				GeoIP:  geoip.NewMock(t, r),
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			got := c.getASNumber(tc.Flow, net.ParseIP(tc.Addr))
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("getASNumber() (-got, +want):\n%s", diff)
			}
		})
	}
}
