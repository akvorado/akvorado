// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"bytes"
	"encoding/gob"
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	commoncgnat "akvorado/common/cgnat"
	"akvorado/common/daemon"
	"akvorado/common/httpserver"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	outletcgnat "akvorado/outlet/cgnat"
	"akvorado/outlet/clickhouse"
	"akvorado/outlet/flow"
	"akvorado/outlet/kafka"
	"akvorado/outlet/metadata"
	"akvorado/outlet/routing"
	"akvorado/outlet/routing/provider"
)

func TestCGNATBidirectionalEnrichment(t *testing.T) {
	cases := []struct {
		name      string
		srcAddr   netip.Addr
		dstAddr   netip.Addr
		srcPort   uint16
		dstPort   uint16
		matchedOn string
	}{
		{
			name:      "source side",
			srcAddr:   netip.MustParseAddr("62.45.100.176"),
			dstAddr:   netip.MustParseAddr("1.1.1.1"),
			srcPort:   12000,
			dstPort:   443,
			matchedOn: "src",
		},
		{
			name:      "destination fallback",
			srcAddr:   netip.MustParseAddr("198.51.100.1"),
			dstAddr:   netip.MustParseAddr("62.45.100.176"),
			srcPort:   443,
			dstPort:   12000,
			matchedOn: "dst",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := reporter.NewMock(t)
			daemonComponent := daemon.NewMock(t)
			metadataComponent := metadata.NewMock(t, r, metadata.DefaultConfiguration(), metadata.Dependencies{Daemon: daemonComponent})
			flowComponent, err := flow.New(r, flow.DefaultConfiguration(), flow.Dependencies{Schema: schema.NewMock(t)})
			if err != nil {
				t.Fatalf("flow.New() error:\n%+v", err)
			}
			httpComponent := httpserver.NewMock(t, r)
			routingComponent := routing.NewMock(t, r)
			routingComponent.PopulateRIB(t)
			kafkaComponent, incoming := kafka.NewMock(t, kafka.DefaultConfiguration())
			cgnatComponent, err := outletcgnat.New(r, outletcgnat.DefaultConfiguration(), outletcgnat.Dependencies{Daemon: daemonComponent})
			if err != nil {
				t.Fatalf("cgnat.New() error:\n%+v", err)
			}

			var clickhouseMessages []*schema.FlowMessage
			var clickhouseMessagesMutex sync.Mutex
			clickhouseComponent := clickhouse.NewMock(t, func(msg *schema.FlowMessage) {
				clickhouseMessagesMutex.Lock()
				defer clickhouseMessagesMutex.Unlock()
				clickhouseMessages = append(clickhouseMessages, msg)
			})

			component, err := New(r, DefaultConfiguration(), Dependencies{
				Daemon:     daemonComponent,
				Flow:       flowComponent,
				Metadata:   metadataComponent,
				Routing:    routingComponent,
				CGNAT:      cgnatComponent,
				Kafka:      kafkaComponent,
				ClickHouse: clickhouseComponent,
				HTTP:       httpComponent,
				Schema:     schema.NewMock(t).EnableAllColumns(),
			})
			if err != nil {
				t.Fatalf("core.New() error:\n%+v", err)
			}
			if err := component.Start(); err != nil {
				t.Fatalf("core.Start() error:\n%+v", err)
			}
			t.Cleanup(func() {
				_ = component.Stop()
			})

			allocated, err := commoncgnat.ParseSyslogLine("Jul  6 14:05:37 host NAT:20260706140537 3e2d PortBatchV2Allocated: [100.104.128.32 62.45.100.176 11777 12288]")
			if err != nil {
				t.Fatalf("ParseSyslogLine() error:\n%+v", err)
			}
			cgnatPayload, err := commoncgnat.Encode(allocated)
			if err != nil {
				t.Fatalf("Encode() error:\n%+v", err)
			}

			mappingRawFlow := &pb.RawFlow{
				TimeReceived:  uint64(allocated.Timestamp.Unix()),
				Payload:       cgnatPayload,
				SourceAddress: netip.MustParseAddr("127.0.0.1").AsSlice(),
				Decoder:       pb.RawFlow_DECODER_CGNAT,
			}
			mappingData, err := proto.Marshal(mappingRawFlow)
			if err != nil {
				t.Fatalf("proto.Marshal() mapping error:\n%+v", err)
			}
			incoming <- mappingData

			flowMessage := &schema.FlowMessage{
				TimeReceived:    uint32(allocated.Timestamp.Unix()),
				SamplingRate:    1000,
				ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
				InIf:            100,
				OutIf:           200,
				SrcAddr:         tc.srcAddr,
				DstAddr:         tc.dstAddr,
				SrcPort:         tc.srcPort,
				DstPort:         tc.dstPort,
				OtherColumns: map[schema.ColumnKey]any{
					schema.ColumnEType:   uint32(0x800),
					schema.ColumnProto:   uint32(6),
					schema.ColumnBytes:   uint64(1000),
					schema.ColumnPackets: uint64(4),
					schema.ColumnSrcPort: tc.srcPort,
					schema.ColumnDstPort: tc.dstPort,
				},
			}

			var buf bytes.Buffer
			if err := gob.NewEncoder(&buf).Encode(flowMessage); err != nil {
				t.Fatalf("gob.Encode() error:\n%+v", err)
			}
			flowRawFlow := &pb.RawFlow{
				TimeReceived:  uint64(time.Now().Unix()),
				Payload:       buf.Bytes(),
				SourceAddress: flowMessage.ExporterAddress.AsSlice(),
				Decoder:       pb.RawFlow_DECODER_GOB,
			}
			flowData, err := proto.Marshal(flowRawFlow)
			if err != nil {
				t.Fatalf("proto.Marshal() flow error:\n%+v", err)
			}
			incoming <- flowData

			time.Sleep(100 * time.Millisecond)

			clickhouseMessagesMutex.Lock()
			defer clickhouseMessagesMutex.Unlock()
			if len(clickhouseMessages) == 0 {
				t.Fatal("no flow forwarded to ClickHouse")
			}
			got := clickhouseMessages[len(clickhouseMessages)-1].OtherColumns

			if got[schema.ColumnCGNATMatchedOn] != tc.matchedOn {
				t.Fatalf("matched_on = %v, want %s", got[schema.ColumnCGNATMatchedOn], tc.matchedOn)
			}
			if got[schema.ColumnCGNATPrivateAddr] != allocated.PrivateIP {
				t.Fatalf("private addr = %v, want %v", got[schema.ColumnCGNATPrivateAddr], allocated.PrivateIP)
			}
			if got[schema.ColumnCGNATPublicAddr] != allocated.PublicIP {
				t.Fatalf("public addr = %v, want %v", got[schema.ColumnCGNATPublicAddr], allocated.PublicIP)
			}
		})
	}
}

func TestCGNATSourceRoutingOnPrivateAddr(t *testing.T) {
	const (
		publicASN  = 65001
		privateASN = 65002
	)

	r := reporter.NewMock(t)
	daemonComponent := daemon.NewMock(t)
	metadataComponent := metadata.NewMock(t, r, metadata.DefaultConfiguration(), metadata.Dependencies{Daemon: daemonComponent})
	flowComponent, err := flow.New(r, flow.DefaultConfiguration(), flow.Dependencies{Schema: schema.NewMock(t)})
	if err != nil {
		t.Fatalf("flow.New() error:\n%+v", err)
	}
	httpComponent := httpserver.NewMock(t, r)
	routingComponent := routing.NewCustomMock(t, r, func(_ context.Context, ip, _, _ netip.Addr) (provider.LookupResult, error) {
		switch ip.String() {
		case "62.45.100.176":
			return provider.LookupResult{ASN: publicASN}, nil
		case "100.104.128.32":
			return provider.LookupResult{ASN: privateASN}, nil
		default:
			return provider.LookupResult{}, nil
		}
	})
	kafkaComponent, incoming := kafka.NewMock(t, kafka.DefaultConfiguration())
	cgnatComponent, err := outletcgnat.New(r, outletcgnat.DefaultConfiguration(), outletcgnat.Dependencies{Daemon: daemonComponent})
	if err != nil {
		t.Fatalf("cgnat.New() error:\n%+v", err)
	}

	var clickhouseMessages []*schema.FlowMessage
	var clickhouseMessagesMutex sync.Mutex
	clickhouseComponent := clickhouse.NewMock(t, func(msg *schema.FlowMessage) {
		clickhouseMessagesMutex.Lock()
		defer clickhouseMessagesMutex.Unlock()
		clickhouseMessages = append(clickhouseMessages, msg)
	})

	config := DefaultConfiguration()
	config.RouteSourceOnCGNATPrivateAddr = true
	component, err := New(r, config, Dependencies{
		Daemon:     daemonComponent,
		Flow:       flowComponent,
		Metadata:   metadataComponent,
		Routing:    routingComponent,
		CGNAT:      cgnatComponent,
		Kafka:      kafkaComponent,
		ClickHouse: clickhouseComponent,
		HTTP:       httpComponent,
		Schema:     schema.NewMock(t).EnableAllColumns(),
	})
	if err != nil {
		t.Fatalf("core.New() error:\n%+v", err)
	}
	if err := component.Start(); err != nil {
		t.Fatalf("core.Start() error:\n%+v", err)
	}
	t.Cleanup(func() {
		_ = component.Stop()
	})

	allocated, err := commoncgnat.ParseSyslogLine("Jul  6 14:05:37 host NAT:20260706140537 3e2d PortBatchV2Allocated: [100.104.128.32 62.45.100.176 11777 12288]")
	if err != nil {
		t.Fatalf("ParseSyslogLine() error:\n%+v", err)
	}
	cgnatPayload, err := commoncgnat.Encode(allocated)
	if err != nil {
		t.Fatalf("Encode() error:\n%+v", err)
	}

	mappingRawFlow := &pb.RawFlow{
		TimeReceived:  uint64(allocated.Timestamp.Unix()),
		Payload:       cgnatPayload,
		SourceAddress: netip.MustParseAddr("127.0.0.1").AsSlice(),
		Decoder:       pb.RawFlow_DECODER_CGNAT,
	}
	mappingData, err := proto.Marshal(mappingRawFlow)
	if err != nil {
		t.Fatalf("proto.Marshal() mapping error:\n%+v", err)
	}
	incoming <- mappingData

	flowMessage := &schema.FlowMessage{
		TimeReceived:    uint32(allocated.Timestamp.Unix()),
		SamplingRate:    1000,
		ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.142"),
		InIf:            100,
		OutIf:           200,
		SrcAddr:         netip.MustParseAddr("62.45.100.176"),
		DstAddr:         netip.MustParseAddr("1.1.1.1"),
		SrcPort:         12000,
		DstPort:         443,
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnEType:   uint32(0x800),
			schema.ColumnProto:   uint32(6),
			schema.ColumnBytes:   uint64(1000),
			schema.ColumnPackets: uint64(4),
			schema.ColumnSrcPort: uint16(12000),
			schema.ColumnDstPort: uint16(443),
		},
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(flowMessage); err != nil {
		t.Fatalf("gob.Encode() error:\n%+v", err)
	}
	flowRawFlow := &pb.RawFlow{
		TimeReceived:  uint64(time.Now().Unix()),
		Payload:       buf.Bytes(),
		SourceAddress: flowMessage.ExporterAddress.AsSlice(),
		Decoder:       pb.RawFlow_DECODER_GOB,
	}
	flowData, err := proto.Marshal(flowRawFlow)
	if err != nil {
		t.Fatalf("proto.Marshal() flow error:\n%+v", err)
	}
	incoming <- flowData

	time.Sleep(100 * time.Millisecond)

	clickhouseMessagesMutex.Lock()
	defer clickhouseMessagesMutex.Unlock()
	if len(clickhouseMessages) == 0 {
		t.Fatal("no flow forwarded to ClickHouse")
	}
	got := clickhouseMessages[len(clickhouseMessages)-1]
	if got.SrcAS != privateASN {
		t.Fatalf("SrcAS = %d, want %d", got.SrcAS, privateASN)
	}
	if got.DstAS != 0 {
		t.Fatalf("DstAS = %d, want 0", got.DstAS)
	}
	if got.OtherColumns[schema.ColumnCGNATMatchedOn] != "src" {
		t.Fatalf("matched_on = %v, want src", got.OtherColumns[schema.ColumnCGNATMatchedOn])
	}
}
