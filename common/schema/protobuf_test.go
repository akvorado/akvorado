// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"akvorado/common/helpers"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestProtobufDefinition(t *testing.T) {
	// Use a smaller version
	flows := Schema{
		columns: []Column{
			{
				Key:            ColumnTimeReceived,
				ClickHouseType: "DateTime",
				ProtobufType:   protoreflect.Uint64Kind,
			},
			{Key: ColumnSamplingRate, ClickHouseType: "UInt64"},
			{Key: ColumnExporterAddress, ClickHouseType: "LowCardinality(IPv6)"},
			{Key: ColumnExporterName, ClickHouseType: "LowCardinality(String)"},
			{
				Key:            ColumnSrcAddr,
				ClickHouseType: "IPv6",
			},
			{
				Key:            ColumnSrcNetMask,
				ClickHouseType: "UInt8",
			},
			{
				Key:             ColumnSrcNetPrefix,
				ClickHouseType:  "String",
				ClickHouseAlias: `something`,
			},
			{Key: ColumnSrcAS, ClickHouseType: "UInt32"},
			{
				Key:                    ColumnSrcNetName,
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: fmt.Sprintf("dictGetOrDefault('%s', 'name', SrcAddr, '')", DictionaryNetworks),
			},
			{
				Key:            ColumnDstASPath,
				ClickHouseType: "Array(UInt32)",
			},
			{
				Key:            ColumnDstLargeCommunities,
				ClickHouseType: "Array(UInt128)",
				ClickHouseTransformFrom: []Column{
					{Key: ColumnDstLargeCommunitiesASN, ClickHouseType: "Array(UInt32)"},
					{Key: ColumnDstLargeCommunitiesLocalData1, ClickHouseType: "Array(UInt32)"},
					{Key: ColumnDstLargeCommunitiesLocalData2, ClickHouseType: "Array(UInt32)"},
				},
				ClickHouseTransformTo: "something",
			},
			{Key: ColumnInIfName, ClickHouseType: "LowCardinality(String)"},
			{
				Key:                     ColumnInIfBoundary,
				ClickHouseType:          "Enum8('undefined' = 0, 'external' = 1, 'internal' = 2)",
				ClickHouseNotSortingKey: true,
				ProtobufType:            protoreflect.EnumKind,
				ProtobufEnumName:        "Boundary",
				ProtobufEnum: map[int]string{
					0: "UNDEFINED",
					1: "EXTERNAL",
					2: "INTERNAL",
				},
			},
			{Key: ColumnBytes, ClickHouseType: "UInt64"},
		},
	}.finalize()

	got := flows.ProtobufDefinition()
	expected := `
syntax = "proto3";

message FlowMessagev5WRSGBXQDXZSUHZQE6QEHLI5JM {
 enum Boundary { UNDEFINED = 0; EXTERNAL = 1; INTERNAL = 2; }

 uint64 TimeReceived = 1;
 uint64 SamplingRate = 2;
 bytes ExporterAddress = 3;
 string ExporterName = 4;
 bytes SrcAddr = 5;
 bytes DstAddr = 6;
 uint32 SrcNetMask = 7;
 uint32 DstNetMask = 8;
 uint32 SrcAS = 9;
 uint32 DstAS = 10;
 repeated uint32 DstASPath = 11;
 repeated uint32 DstLargeCommunitiesASN = 12;
 repeated uint32 DstLargeCommunitiesLocalData1 = 13;
 repeated uint32 DstLargeCommunitiesLocalData2 = 14;
 string InIfName = 15;
 string OutIfName = 16;
 Boundary InIfBoundary = 17;
 Boundary OutIfBoundary = 18;
 uint64 Bytes = 19;
}
`
	if diff := helpers.Diff(strings.Split(got, "\n"), strings.Split(expected, "\n")); diff != "" {
		t.Fatalf("ProtobufDefinition() (-got, +want): %s", diff)
	}
}

func TestProtobufMarshal(t *testing.T) {
	c := NewMock(t)
	exporterAddress := netip.MustParseAddr("::ffff:203.0.113.14")
	bf := &FlowMessage{}
	bf.TimeReceived = 1000
	bf.SamplingRate = 20000
	bf.ExporterAddress = exporterAddress
	c.ProtobufAppendVarint(bf, ColumnDstAS, 65000)
	c.ProtobufAppendVarint(bf, ColumnBytes, 200)
	c.ProtobufAppendVarint(bf, ColumnPackets, 300)
	c.ProtobufAppendVarint(bf, ColumnBytes, 300) // duplicate!

	got := c.ProtobufMarshal(bf)

	size, n := protowire.ConsumeVarint(got)
	if uint64(len(got)-n) != size {
		t.Fatalf("ProtobufMarshal() produced an incorrect size: %d + %d != %d", size, n, len(got))
	}

	// text schema definition for reference
	// syntax = "proto3";

	// message FlowMessagevLAABIGYMRYZPTGOYIIFZNYDEQM {
	// enum Boundary { UNDEFINED = 0; EXTERNAL = 1; INTERNAL = 2; }

	// uint64 TimeReceived = 1;
	// uint64 SamplingRate = 2;
	// bytes ExporterAddress = 3;
	// string ExporterName = 4;
	// string ExporterGroup = 5;
	// string ExporterRole = 6;
	// string ExporterSite = 7;
	// string ExporterRegion = 8;
	// string ExporterTenant = 9;
	// bytes SrcAddr = 10;
	// bytes DstAddr = 11;
	// uint32 SrcNetMask = 12;
	// uint32 DstNetMask = 13;
	// uint32 SrcAS = 14;
	// uint32 DstAS = 15;
	// repeated uint32 DstASPath = 18;
	// repeated uint32 DstCommunities = 19;
	// repeated uint32 DstLargeCommunitiesASN = 20;
	// repeated uint32 DstLargeCommunitiesLocalData1 = 21;
	// repeated uint32 DstLargeCommunitiesLocalData2 = 22;
	// string InIfName = 23;
	// string OutIfName = 24;
	// string InIfDescription = 25;
	// string OutIfDescription = 26;
	// uint32 InIfSpeed = 27;
	// uint32 OutIfSpeed = 28;
	// string InIfConnectivity = 29;
	// string OutIfConnectivity = 30;
	// string InIfProvider = 31;
	// string OutIfProvider = 32;
	// Boundary InIfBoundary = 33;
	// Boundary OutIfBoundary = 34;
	// uint32 EType = 35;
	// uint32 Proto = 36;
	// uint32 SrcPort = 37;
	// uint32 DstPort = 38;
	// uint64 Bytes = 39;
	// uint64 Packets = 40;
	// uint32 ForwardingStatus = 41;
	// }
	// to check: https://protobuf-decoder.netlify.app/
	t.Run("compare as bytes", func(t *testing.T) {
		expected := []byte{
			// DstAS
			// 15: 65000
			0x78, 0xe8, 0xfb, 0x03,
			// Bytes
			// 39: 200
			0xb8, 0x02, 0xc8, 0x01,
			// Packet
			// 40: 300
			0xc0, 0x02, 0xac, 0x02,
			// TimeReceived
			// 1: 1000
			0x08, 0xe8, 0x07,
			// SamplingRate
			// 2: 20000
			0x10, 0xa0, 0x9c, 0x01,
			// ExporterAddress
			// 3: ::ffff:203.0.113.14
			0x1a, 0x10, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xff, 0xff, 0xcb, 0x0, 0x71, 0xe,
		}
		if diff := helpers.Diff(got[n:], expected); diff != "" {
			t.Logf("got: %v", got)

			t.Fatalf("ProtobufMarshal() (-got, +want):\n%s", diff)
		}
	})

	t.Run("compare as protobuf message", func(t *testing.T) {
		got := c.ProtobufDecode(t, got)
		expected := FlowMessage{
			TimeReceived:    1000,
			SamplingRate:    20000,
			ExporterAddress: exporterAddress,
			DstAS:           65000,
			ProtobufDebug: map[ColumnKey]interface{}{
				ColumnBytes:   200,
				ColumnPackets: 300,
			},
		}
		if diff := helpers.Diff(got, expected); diff != "" {
			t.Fatalf("ProtobufDecode() (-got, +want):\n%s", diff)
		}
	})
}

func BenchmarkProtobufMarshal(b *testing.B) {
	c := NewMock(b)
	exporterAddress := netip.MustParseAddr("::ffff:203.0.113.14")
	DisableDebug(b)
	for range b.N {
		bf := &FlowMessage{
			TimeReceived:    1000,
			SamplingRate:    20000,
			ExporterAddress: exporterAddress,
		}
		c.ProtobufAppendVarint(bf, ColumnDstAS, 65000)
		c.ProtobufAppendVarint(bf, ColumnBytes, 200)
		c.ProtobufAppendVarint(bf, ColumnPackets, 300)
		c.ProtobufAppendVarint(bf, ColumnBytes, 300)    // duplicate!
		c.ProtobufAppendVarint(bf, ColumnSrcVlan, 1600) // disabled!
		c.ProtobufMarshal(bf)
	}
}
