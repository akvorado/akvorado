// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/flow"
	"akvorado/inlet/geoip"
	"akvorado/inlet/kafka"
	"akvorado/inlet/metadata"
	"akvorado/inlet/routing"
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
			Name: "exporter rule with an error",
			Configuration: gin.H{
				"exporterclassifiers": []string{
					`ClassifyTenant("alfred")`,
					`Exporter.Name > "hello"`,
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
					schema.ColumnExporterTenant:   "alfred",
					schema.ColumnInIfName:         "Gi0/0/100",
					schema.ColumnOutIfName:        "Gi0/0/200",
					schema.ColumnInIfDescription:  "Interface 100",
					schema.ColumnOutIfDescription: "Interface 200",
					schema.ColumnInIfSpeed:        1000,
					schema.ColumnOutIfSpeed:       1000,
				},
			},
		}, {
			Name: "exporter rule with reject",
			Configuration: gin.H{
				"exporterclassifiers": []string{
					`Reject()`,
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
			OutputFlow: nil,
		}, {
			Name: "interface rule with reject",
			Configuration: gin.H{
				"interfaceclassifiers": []string{
					`Reject()`,
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
			OutputFlow: nil,
		}, {
			Name: "interface rule with index",
			Configuration: gin.H{
				"interfaceclassifiers": []string{
					`Interface.Index == 100 && ClassifyProvider("index1")`,
					`Interface.Index == 200 && ClassifyProvider("index2")`,
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
					schema.ColumnInIfProvider:     "index1",
					schema.ColumnOutIfProvider:    "index2",
					schema.ColumnInIfName:         "Gi0/0/100",
					schema.ColumnOutIfName:        "Gi0/0/200",
					schema.ColumnInIfDescription:  "Interface 100",
					schema.ColumnOutIfDescription: "Interface 200",
					schema.ColumnInIfSpeed:        1000,
					schema.ColumnOutIfSpeed:       1000,
				},
			},
		}, {
			Name: "interface rule with rename",
			Configuration: gin.H{
				"interfaceclassifiers": []string{
					`Interface.Name == "Gi0/0/100" && SetName("eth100")`,
					`Interface.Name == "Gi0/0/200" && SetDescription("Super Speed")`,
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
					schema.ColumnInIfName:         "eth100",
					schema.ColumnOutIfName:        "Gi0/0/200",
					schema.ColumnInIfDescription:  "Interface 100",
					schema.ColumnOutIfDescription: "Super Speed",
					schema.ColumnInIfSpeed:        1000,
					schema.ColumnOutIfSpeed:       1000,
				},
			},
		}, {
			Name: "interface rule with VLAN",
			Configuration: gin.H{
				"interfaceclassifiers": []string{
					`Interface.VLAN > 200 && SetName(Format("%s.%d", Interface.Name, Interface.VLAN))`,
				},
			},
			InputFlow: func() *schema.FlowMessage {
				return &schema.FlowMessage{
					SamplingRate:    1000,
					ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
					InIf:            100,
					SrcVlan:         10,
					OutIf:           200,
					DstVlan:         300,
				}
			},
			OutputFlow: &schema.FlowMessage{
				SamplingRate:    1000,
				ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnExporterName:     "192_0_2_142",
					schema.ColumnInIfName:         "Gi0/0/100",
					schema.ColumnOutIfName:        "Gi0/0/200.300",
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
			Name:          "use data from routing",
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
					schema.ColumnSrcNetMask:                    27,
					schema.ColumnDstNetMask:                    27,
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			r := reporter.NewMock(t)

			// Prepare all components.
			daemonComponent := daemon.NewMock(t)
			metadataComponent := metadata.NewMock(t, r, metadata.DefaultConfiguration(),
				metadata.Dependencies{Daemon: daemonComponent})
			flowComponent := flow.NewMock(t, r, flow.DefaultConfiguration())
			geoipComponent := geoip.NewMock(t, r)
			kafkaComponent, kafkaProducer := kafka.NewMock(t, r, kafka.DefaultConfiguration())
			httpComponent := httpserver.NewMock(t, r)
			routingComponent := routing.NewMock(t, r)
			routingComponent.PopulateRIB(t)

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
				Daemon:   daemonComponent,
				Flow:     flowComponent,
				Metadata: metadataComponent,
				GeoIP:    geoipComponent,
				Kafka:    kafkaComponent,
				HTTP:     httpComponent,
				Routing:  routingComponent,
				Schema:   schema.NewMock(t),
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, c)

			// Inject twice since otherwise, we get a cache miss
			received := make(chan bool)
			if tc.OutputFlow != nil {
				kafkaProducer.ExpectInputWithMessageCheckerFunctionAndSucceed(
					func(msg *sarama.ProducerMessage) error {
						defer close(received)
						b, err := msg.Value.Encode()
						if err != nil {
							t.Fatalf("Kafka message encoding error:\n%+v", err)
						}
						t.Logf("Raw message: %v", b)
						got := c.d.Schema.ProtobufDecode(t, b)
						if diff := helpers.Diff(&got, tc.OutputFlow); diff != "" {
							t.Errorf("Classifier (-got, +want):\n%s", diff)
						}
						return nil
					})
			}
			// Else, we should not get a message, but that's not possible to test.
			flowComponent.Inject(tc.InputFlow())
			time.Sleep(50 * time.Millisecond) // Needed to let poller does its job
			flowComponent.Inject(tc.InputFlow())
			if tc.OutputFlow != nil {
				select {
				case <-received:
				case <-time.After(1 * time.Second):
					t.Fatal("Kafka message not received")
				}
			} else {
				time.Sleep(100 * time.Millisecond)
			}
			gotMetrics := r.GetMetrics("akvorado_inlet_core_flows_", "-processing_")
			expectedMetrics := map[string]string{
				`errors{error="SNMP cache miss",exporter="192.0.2.142"}`: "1",
				`http_clients`:                     "0",
				`received{exporter="192.0.2.142"}`: "2",
			}
			if tc.OutputFlow != nil {
				expectedMetrics[`forwarded{exporter="192.0.2.142"}`] = "1"
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
		{"1.0.0.1", 12322, 0, []ASNProvider{ASNProviderFlow}, 12322},
		{"::ffff:1.0.0.1", 12322, 0, []ASNProvider{ASNProviderFlow}, 12322},
		{"1.0.0.1", 65536, 0, []ASNProvider{ASNProviderFlow}, 65536},
		{"1.0.0.1", 65536, 0, []ASNProvider{ASNProviderFlowExceptPrivate}, 0},
		{"1.0.0.1", 4_200_000_121, 0, []ASNProvider{ASNProviderFlowExceptPrivate}, 0},
		{"1.0.0.1", 65536, 0, []ASNProvider{ASNProviderFlowExceptPrivate, ASNProviderFlow}, 65536},
		{"1.0.0.1", 12322, 0, []ASNProvider{ASNProviderFlowExceptPrivate}, 12322},
		{"1.0.0.1", 12322, 0, []ASNProvider{ASNProviderGeoIP}, 15169},
		{"2.0.0.1", 12322, 0, []ASNProvider{ASNProviderGeoIP}, 0},
		{"1.0.0.1", 12322, 0, []ASNProvider{ASNProviderGeoIP, ASNProviderFlow}, 15169},
		// 10
		{"1.0.0.1", 12322, 0, []ASNProvider{ASNProviderFlow, ASNProviderGeoIP}, 12322},
		{"2.0.0.1", 12322, 0, []ASNProvider{ASNProviderFlow, ASNProviderGeoIP}, 12322},
		{"2.0.0.1", 12322, 0, []ASNProvider{ASNProviderGeoIP, ASNProviderFlow}, 12322},
		{"192.0.2.2", 12322, 174, []ASNProvider{ASNProviderRouting}, 174},
		{"192.0.2.129", 12322, 1299, []ASNProvider{ASNProviderRouting}, 1299},
		{"192.0.2.254", 12322, 0, []ASNProvider{ASNProviderRouting}, 0},
		{"1.0.0.1", 12322, 65300, []ASNProvider{ASNProviderRouting}, 65300},
		{"1.0.0.1", 12322, 15169, []ASNProvider{ASNProviderRoutingExceptPrivate, ASNProviderGeoIP}, 15169},
	}
	for i, tc := range cases {
		i++
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			r := reporter.NewMock(t)

			// We don't need all components as we won't start the component.
			configuration := DefaultConfiguration()
			configuration.ASNProviders = tc.Providers
			routingComponent := routing.NewMock(t, r)
			routingComponent.PopulateRIB(t)

			c, err := New(r, configuration, Dependencies{
				Daemon:  daemon.NewMock(t),
				GeoIP:   geoip.NewMock(t, r),
				Routing: routingComponent,
				Schema:  schema.NewMock(t),
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

func TestGetNetMask(t *testing.T) {
	cases := []struct {
		//		Addr        string
		FlowNetMask uint8
		BMPNetMask  uint8
		Providers   []NetProvider
		Expected    uint8
	}{
		// Flow
		{0, 0, []NetProvider{NetProviderFlow}, 0},
		{32, 0, []NetProvider{NetProviderFlow}, 32},
		{0, 16, []NetProvider{NetProviderFlow}, 0},
		// BMP
		{0, 0, []NetProvider{NetProviderRouting}, 0},
		{32, 12, []NetProvider{NetProviderRouting}, 12},
		{0, 16, []NetProvider{NetProviderRouting}, 16},
		{24, 0, []NetProvider{NetProviderRouting}, 0},
		// Both, the first provider with a non-default route is taken
		{0, 0, []NetProvider{NetProviderRouting, NetProviderFlow}, 0},
		{12, 0, []NetProvider{NetProviderRouting, NetProviderFlow}, 12},
		{0, 13, []NetProvider{NetProviderRouting, NetProviderFlow}, 13},
		{12, 0, []NetProvider{NetProviderRouting, NetProviderFlow}, 12},
		{12, 24, []NetProvider{NetProviderRouting, NetProviderFlow}, 24},

		{0, 0, []NetProvider{NetProviderFlow, NetProviderRouting}, 0},
		{12, 0, []NetProvider{NetProviderFlow, NetProviderRouting}, 12},
		{0, 13, []NetProvider{NetProviderFlow, NetProviderRouting}, 13},
		{12, 0, []NetProvider{NetProviderFlow, NetProviderRouting}, 12},
		{12, 24, []NetProvider{NetProviderFlow, NetProviderRouting}, 12},
	}
	for i, tc := range cases {
		i++
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			r := reporter.NewMock(t)

			// We don't need all components as we won't start the component.
			configuration := DefaultConfiguration()
			configuration.NetProviders = tc.Providers
			routingComponent := routing.NewMock(t, r)
			routingComponent.PopulateRIB(t)

			c, err := New(r, configuration, Dependencies{
				Daemon:  daemon.NewMock(t),
				GeoIP:   geoip.NewMock(t, r),
				Routing: routingComponent,
				Schema:  schema.NewMock(t),
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			got := c.getNetMask(tc.FlowNetMask, uint8(tc.BMPNetMask))
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("getNetMask() (-got, +want):\n%s", diff)
			}
		})
	}
}
