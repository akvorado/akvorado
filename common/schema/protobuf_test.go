// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
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
				ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'name', SrcAddr, '')",
			},
			{Key: ColumnSrcCountry, ClickHouseType: "FixedString(2)"},
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

message FlowMessagevLH2TTFF7P352DSYYCJYWFCXHAM {
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
 string SrcCountry = 11;
 string DstCountry = 12;
 repeated uint32 DstASPath = 13;
 repeated uint32 DstLargeCommunitiesASN = 14;
 repeated uint32 DstLargeCommunitiesLocalData1 = 15;
 repeated uint32 DstLargeCommunitiesLocalData2 = 16;
 string InIfName = 17;
 string OutIfName = 18;
 Boundary InIfBoundary = 19;
 Boundary OutIfBoundary = 20;
 uint64 Bytes = 21;
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
	c.ProtobufAppendBytes(bf, ColumnDstCountry, []byte("FR"))

	got := c.ProtobufMarshal(bf)

	size, n := protowire.ConsumeVarint(got)
	if uint64(len(got)-n) != size {
		t.Fatalf("ProtobufMarshal() produced an incorrect size: %d + %d != %d", size, n, len(got))
	}

	t.Run("compare as bytes", func(t *testing.T) {
		expected := []byte{
			// 15: 65000
			0x78, 0xe8, 0xfb, 0x03,
			// 41: 200
			0xc8, 0x02, 0xc8, 0x01,
			// 42: 300
			0xd0, 0x02, 0xac, 0x02,
			// 19: FR
			0x9a, 0x01, 0x02, 0x46, 0x52,
			// 1: 1000
			0x08, 0xe8, 0x07,
			// 2: 20000
			0x10, 0xa0, 0x9c, 0x01,
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
				ColumnBytes:      200,
				ColumnPackets:    300,
				ColumnDstCountry: "FR",
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
	for i := 0; i < b.N; i++ {
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
		c.ProtobufAppendBytes(bf, ColumnDstCountry, []byte("FR"))
		c.ProtobufMarshal(bf)
	}
}
