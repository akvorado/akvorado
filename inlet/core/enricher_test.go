// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"fmt"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/bmp"
	"akvorado/inlet/flow"
	"akvorado/inlet/geoip"
	"akvorado/inlet/kafka"
	"akvorado/inlet/snmp"
)

func TestEnrich(t *testing.T) {
	cases := []struct {
		Name          string
		Configuration gin.H
		InputFlow     func() *schema.FlowMessage
		OutputFlow    *schema.FlowMessage
	}{
		{
			Name:          "no rule",
			Configuration: gin.H{},
			InputFlow: func() *schema.FlowMessage {
				return &schema.FlowMessage{
					SamplingRate:    1000,
					ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &schema.FlowMessage{
				SamplingRate:    1000,
				ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnExporterName:     "192_0_2_142",
					schema.ColumnInIfName:         "Gi0/0/100",
					schema.ColumnOutIfName:        "Gi0/0/200",
					schema.ColumnInIfDescription:  "Interface 100",
					schema.ColumnOutIfDescription: "Interface 200",
					schema.ColumnInIfSpeed:        1000,
					schema.ColumnOutIfSpeed:       1000,
				},
			},
		}, {
			Name: "no rule, override sampling rate",
			Configuration: gin.H{"overridesamplingrate": gin.H{
				"192.0.2.0/24":   100,
				"192.0.2.128/25": 500,
				"192.0.2.141/32": 1000,
			}},
			InputFlow: func() *schema.FlowMessage {
				return &schema.FlowMessage{
					SamplingRate:    1000,
					ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &schema.FlowMessage{
				SamplingRate:    500,
				ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnExporterName:     "192_0_2_142",
					schema.ColumnInIfName:         "Gi0/0/100",
					schema.ColumnOutIfName:        "Gi0/0/200",
					schema.ColumnInIfDescription:  "Interface 100",
					schema.ColumnOutIfDescription: "Interface 200",
					schema.ColumnInIfSpeed:        1000,
					schema.ColumnOutIfSpeed:       1000,
				},
			},
		}, {
			Name:          "no rule, no sampling rate, default is one value",
			Configuration: gin.H{"defaultsamplingrate": 500},
			InputFlow: func() *schema.FlowMessage {
				return &schema.FlowMessage{
					ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &schema.FlowMessage{
				SamplingRate:    500,
				ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnExporterName:     "192_0_2_142",
					schema.ColumnInIfName:         "Gi0/0/100",
					schema.ColumnOutIfName:        "Gi0/0/200",
					schema.ColumnInIfDescription:  "Interface 100",
					schema.ColumnOutIfDescription: "Interface 200",
					schema.ColumnInIfSpeed:        1000,
					schema.ColumnOutIfSpeed:       1000,
				},
			},
		}, {
			Name: "no rule, no sampling rate, default is map",
			Configuration: gin.H{"defaultsamplingrate": gin.H{
				"192.0.2.0/24":   100,
				"192.0.2.128/25": 500,
				"192.0.2.141/32": 1000,
			}},
			InputFlow: func() *schema.FlowMessage {
				return &schema.FlowMessage{
					ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &schema.FlowMessage{
				SamplingRate:    500,
				ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnExporterName:     "192_0_2_142",
					schema.ColumnInIfName:         "Gi0/0/100",
					schema.ColumnOutIfName:        "Gi0/0/200",
					schema.ColumnInIfDescription:  "Interface 100",
					schema.ColumnOutIfDescription: "Interface 200",
					schema.ColumnInIfSpeed:        1000,
					schema.ColumnOutIfSpeed:       1000,
				},
			},
		}, {
			Name: "exporter rule",
			Configuration: gin.H{
				"exporterclassifiers": []string{
					`Exporter.Name startsWith "hello" && ClassifyRegion("europe")`,
					`Exporter.Name startsWith "192_" && ClassifyRegion("asia")`,
					`ClassifyRegion("other") && ClassifySite("unknown") && ClassifyTenant("alfred")`,
				},
			},
			InputFlow: func() *schema.FlowMessage {
				return &schema.FlowMessage{
					SamplingRate:    1000,
					ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &schema.FlowMessage{
				SamplingRate:    1000,
				ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnExporterName:     "192_0_2_142",
					schema.ColumnExporterRegion:   "asia",
					schema.ColumnExporterTenant:   "alfred",
					schema.ColumnExporterSite:     "unknown",
					schema.ColumnInIfName:         "Gi0/0/100",
					schema.ColumnOutIfName:        "Gi0/0/200",
					schema.ColumnInIfDescription:  "Interface 100",
					schema.ColumnOutIfDescription: "Interface 200",
					schema.ColumnInIfSpeed:        1000,
					schema.ColumnOutIfSpeed:       1000,
				},
			},
		}, {
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
			InputFlow: func() *schema.FlowMessage {
				return &schema.FlowMessage{
					SamplingRate:    1000,
					ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &schema.FlowMessage{
				SamplingRate:    1000,
				ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnExporterName:     "192_0_2_142",
					schema.ColumnInIfName:         "Gi0/0/100",
					schema.ColumnOutIfName:        "Gi0/0/200",
					schema.ColumnInIfDescription:  "Interface 100",
					schema.ColumnOutIfDescription: "Interface 200",
					schema.ColumnInIfSpeed:        1000,
					schema.ColumnOutIfSpeed:       1000,
					schema.ColumnInIfBoundary:     internalBoundary,
					schema.ColumnOutIfBoundary:    internalBoundary,
				},
			},
		}, {
			Name: "configure twice boundary",
			Configuration: gin.H{
				"interfaceclassifiers": []string{
					`ClassifyInternal()`,
					`ClassifyExternal()`,
				},
			},
			InputFlow: func() *schema.FlowMessage {
				return &schema.FlowMessage{
					SamplingRate:    1000,
					ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &schema.FlowMessage{
				SamplingRate:    1000,
				ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnExporterName:     "192_0_2_142",
					schema.ColumnInIfName:         "Gi0/0/100",
					schema.ColumnOutIfName:        "Gi0/0/200",
					schema.ColumnInIfDescription:  "Interface 100",
					schema.ColumnOutIfDescription: "Interface 200",
					schema.ColumnInIfSpeed:        1000,
					schema.ColumnOutIfSpeed:       1000,
					schema.ColumnInIfBoundary:     2, // Internal
					schema.ColumnOutIfBoundary:    2,
				},
			},
		}, {
			Name: "configure twice provider",
			Configuration: gin.H{
				"interfaceclassifiers": []string{
					`ClassifyProvider("telia")`,
					`ClassifyProvider("cogent")`,
				},
			},
			InputFlow: func() *schema.FlowMessage {
				return &schema.FlowMessage{
					SamplingRate:    1000,
					ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &schema.FlowMessage{
				SamplingRate:    1000,
				ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnExporterName:     "192_0_2_142",
					schema.ColumnInIfName:         "Gi0/0/100",
					schema.ColumnOutIfName:        "Gi0/0/200",
					schema.ColumnInIfDescription:  "Interface 100",
					schema.ColumnOutIfDescription: "Interface 200",
					schema.ColumnInIfSpeed:        1000,
					schema.ColumnOutIfSpeed:       1000,
					schema.ColumnInIfProvider:     "telia",
					schema.ColumnOutIfProvider:    "telia",
				},
			},
		}, {
			Name: "classify depending on description",
			Configuration: gin.H{
				"interfaceclassifiers": []string{
					`ClassifyProvider("Othello")`,
					`ClassifyConnectivityRegex(Interface.Description, " (1\\d+)$", "P$1") && ClassifyExternal()`,
					`ClassifyInternal() && ClassifyConnectivity("core")`,
				},
			},
			InputFlow: func() *schema.FlowMessage {
				return &schema.FlowMessage{
					SamplingRate:    1000,
					ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
					InIf:            100,
					OutIf:           200,
				}
			},
			OutputFlow: &schema.FlowMessage{
				SamplingRate:    1000,
				ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnExporterName:      "192_0_2_142",
					schema.ColumnInIfName:          "Gi0/0/100",
					schema.ColumnOutIfName:         "Gi0/0/200",
					schema.ColumnInIfDescription:   "Interface 100",
					schema.ColumnOutIfDescription:  "Interface 200",
					schema.ColumnInIfSpeed:         1000,
					schema.ColumnOutIfSpeed:        1000,
					schema.ColumnInIfConnectivity:  "p100",
					schema.ColumnOutIfConnectivity: "core",
					schema.ColumnInIfProvider:      "othello",
					schema.ColumnOutIfProvider:     "othello",
					schema.ColumnInIfBoundary:      1, // external
					schema.ColumnOutIfBoundary:     2, // internal
				},
			},
		}, {
			Name:          "use data from BMP",
			Configuration: gin.H{},
			InputFlow: func() *schema.FlowMessage {
				return &schema.FlowMessage{
					SamplingRate:    1000,
					ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
					InIf:            100,
					OutIf:           200,
					SrcAddr:         netip.MustParseAddr("::ffff:192.0.2.142"),
					DstAddr:         netip.MustParseAddr("::ffff:192.0.2.10"),
				}
			},
			OutputFlow: &schema.FlowMessage{
				SamplingRate:    1000,
				ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
				SrcAddr:         netip.MustParseAddr("::ffff:192.0.2.142"),
				DstAddr:         netip.MustParseAddr("::ffff:192.0.2.10"),
				SrcAS:           1299,
				DstAS:           174,
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnExporterName:                  "192_0_2_142",
					schema.ColumnInIfName:                      "Gi0/0/100",
					schema.ColumnOutIfName:                     "Gi0/0/200",
					schema.ColumnInIfDescription:               "Interface 100",
					schema.ColumnOutIfDescription:              "Interface 200",
					schema.ColumnInIfSpeed:                     1000,
					schema.ColumnOutIfSpeed:                    1000,
					schema.ColumnDstASPath:                     []uint32{64200, 1299, 174},
					schema.ColumnDstCommunities:                []uint32{100, 200, 400},
					schema.ColumnDstLargeCommunitiesASN:        []int32{64200},
					schema.ColumnDstLargeCommunitiesLocalData1: []int32{2},
					schema.ColumnDstLargeCommunitiesLocalData2: []int32{3},
				},
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
			bmpComponent, _ := bmp.NewMock(t, r, bmp.DefaultConfiguration())
			bmpComponent.PopulateRIB(t)

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
				SNMP:   snmpComponent,
				GeoIP:  geoipComponent,
				Kafka:  kafkaComponent,
				HTTP:   httpComponent,
				BMP:    bmpComponent,
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
					b, err := msg.Value.Encode()
					if err != nil {
						t.Fatalf("Kafka message encoding error:\n%+v", err)
					}
					t.Logf("Raw message: %v", b)
					got := schema.Flows.ProtobufDecode(t, b)
					if diff := helpers.Diff(&got, tc.OutputFlow, helpers.DiffFormatter(reflect.TypeOf(schema.ColumnBytes), fmt.Sprint)); diff != "" {
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
			gotMetrics := r.GetMetrics("akvorado_inlet_core_flows_", "-processing_")
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
		Addr      string
		FlowAS    uint32
		BMPAS     uint32
		Providers []ASNProvider
		Expected  uint32
	}{
		// 1
		{"1.0.0.1", 12322, 0, []ASNProvider{ProviderFlow}, 12322},
		{"::ffff:1.0.0.1", 12322, 0, []ASNProvider{ProviderFlow}, 12322},
		{"1.0.0.1", 65536, 0, []ASNProvider{ProviderFlow}, 65536},
		{"1.0.0.1", 65536, 0, []ASNProvider{ProviderFlowExceptPrivate}, 0},
		{"1.0.0.1", 4_200_000_121, 0, []ASNProvider{ProviderFlowExceptPrivate}, 0},
		{"1.0.0.1", 65536, 0, []ASNProvider{ProviderFlowExceptPrivate, ProviderFlow}, 65536},
		{"1.0.0.1", 12322, 0, []ASNProvider{ProviderFlowExceptPrivate}, 12322},
		{"1.0.0.1", 12322, 0, []ASNProvider{ProviderGeoIP}, 15169},
		{"2.0.0.1", 12322, 0, []ASNProvider{ProviderGeoIP}, 0},
		{"1.0.0.1", 12322, 0, []ASNProvider{ProviderGeoIP, ProviderFlow}, 15169},
		// 10
		{"1.0.0.1", 12322, 0, []ASNProvider{ProviderFlow, ProviderGeoIP}, 12322},
		{"2.0.0.1", 12322, 0, []ASNProvider{ProviderFlow, ProviderGeoIP}, 12322},
		{"2.0.0.1", 12322, 0, []ASNProvider{ProviderGeoIP, ProviderFlow}, 12322},
		{"192.0.2.2", 12322, 174, []ASNProvider{ProviderBMP}, 174},
		{"192.0.2.129", 12322, 1299, []ASNProvider{ProviderBMP}, 1299},
		{"192.0.2.254", 12322, 0, []ASNProvider{ProviderBMP}, 0},
		{"1.0.0.1", 12322, 65300, []ASNProvider{ProviderBMP}, 65300},
		{"1.0.0.1", 12322, 15169, []ASNProvider{ProviderBMPExceptPrivate, ProviderGeoIP}, 15169},
	}
	for i, tc := range cases {
		i++
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			r := reporter.NewMock(t)

			// We don't need all components as we won't start the component.
			configuration := DefaultConfiguration()
			configuration.ASNProviders = tc.Providers
			bmpComponent, _ := bmp.NewMock(t, r, bmp.DefaultConfiguration())
			bmpComponent.PopulateRIB(t)

			c, err := New(r, configuration, Dependencies{
				Daemon: daemon.NewMock(t),
				GeoIP:  geoip.NewMock(t, r),
				BMP:    bmpComponent,
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			got := c.getASNumber(netip.MustParseAddr(tc.Addr), tc.FlowAS, tc.BMPAS)
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("getASNumber() (-got, +want):\n%s", diff)
			}
		})
	}
}
