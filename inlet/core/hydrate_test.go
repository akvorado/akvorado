// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"net"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/golang/protobuf/proto"
	"gopkg.in/yaml.v2"

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
		Configuration string
		InputFlow     func() *flow.Message
		OutputFlow    *flow.Message
	}{
		{
			Name:          "no rule",
			Configuration: `{}`,
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
			Name: "no rule, no sampling rate",
			Configuration: `
defaultsamplingrate: 500
`,
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
			Configuration: `
exporterclassifiers:
  - Exporter.Name startsWith "hello" && Classify("europe")
  - Exporter.Name startsWith "192_" && Classify("asia")
  - Classify("other")
`,
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
				ExporterGroup:    "asia",
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
			Configuration: `
interfaceclassifiers:
  - >-
     Interface.Description startsWith "Transit:" &&
     ClassifyConnectivity("transit") &&
     ClassifyExternal() &&
     ClassifyProviderRegex(Interface.Description, "^Transit: ([^ ]+)", "$1")
  - ClassifyInternal()
`,
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
			Configuration: `
interfaceclassifiers:
  - ClassifyInternal()
  - ClassifyExternal()
`,
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
			Configuration: `
interfaceclassifiers:
  - ClassifyProvider("telia")
  - ClassifyProvider("cogent")
`,
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
			Configuration: `
interfaceclassifiers:
  - ClassifyProvider("Othello")
  - ClassifyConnectivityRegex(Interface.Description, " (1\\d+)$", "P$1") && ClassifyExternal()
  - ClassifyInternal() && ClassifyConnectivity("core")
`,
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
			if err := yaml.Unmarshal([]byte(tc.Configuration), &configuration); err != nil {
				t.Fatalf("Unmarshal() error:\n%+v", err)
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
