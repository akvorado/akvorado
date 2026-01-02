// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

import (
	"net/netip"
	"path/filepath"
	"testing"

	"akvorado/common/constants"
	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/schema"
)

func TestDecodeMPLSAndIPv4(t *testing.T) {
	sch := schema.NewMock(t).EnableAllColumns()
	pcap := helpers.ReadPcapL2(t, filepath.Join("testdata", "mpls-ipv4.pcap"))
	bf := sch.NewFlowMessage()
	l := ParseEthernet(sch, bf, pb.RawFlow_DECAP_NONE, pcap)
	if l != 40 {
		t.Errorf("ParseEthernet() returned %d, expected 40", l)
	}
	expected := &schema.FlowMessage{
		SrcAddr: netip.MustParseAddr("::ffff:10.31.0.1"),
		DstAddr: netip.MustParseAddr("::ffff:10.34.0.1"),
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnEType:        uint32(constants.ETypeIPv4),
			schema.ColumnProto:        uint32(constants.ProtoTCP),
			schema.ColumnSrcPort:      uint16(11001),
			schema.ColumnDstPort:      uint16(23),
			schema.ColumnTCPFlags:     uint16(16),
			schema.ColumnMPLSLabels:   []uint32{18, 16},
			schema.ColumnIPTTL:        uint8(255),
			schema.ColumnIPTos:        uint8(0xb0),
			schema.ColumnIPFragmentID: uint32(8),
			schema.ColumnSrcMAC:       uint64(0x003096052838),
			schema.ColumnDstMAC:       uint64(0x003096e6fc39),
		},
	}
	if diff := helpers.Diff(bf, expected); diff != "" {
		t.Fatalf("ParseEthernet() (-got, +want):\n%s", diff)
	}
}

func TestDecodeVLANAndIPv6(t *testing.T) {
	sch := schema.NewMock(t).EnableAllColumns()
	pcap := helpers.ReadPcapL2(t, filepath.Join("testdata", "vlan-ipv6.pcap"))
	bf := sch.NewFlowMessage()
	l := ParseEthernet(sch, bf, pb.RawFlow_DECAP_NONE, pcap)
	if l != 179 {
		t.Errorf("ParseEthernet() returned %d, expected 179", l)
	}
	expected := &schema.FlowMessage{
		SrcVlan: 100,
		SrcAddr: netip.MustParseAddr("2402:f000:1:8e01::5555"),
		DstAddr: netip.MustParseAddr("2607:fcd0:100:2300::b108:2a6b"),
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnEType:  uint32(constants.ETypeIPv6),
			schema.ColumnProto:  uint32(constants.ProtoIPv4),
			schema.ColumnIPTTL:  uint8(246),
			schema.ColumnSrcMAC: uint64(0x00121ef2613d),
			schema.ColumnDstMAC: uint64(0xc500000082c4),
		},
	}
	if diff := helpers.Diff(bf, expected); diff != "" {
		t.Fatalf("ParseEthernet() (-got, +want):\n%s", diff)
	}
}

func TestDecodeIPv4IPv4(t *testing.T) {
	sch := schema.NewMock(t).EnableAllColumns()
	pcap := helpers.ReadPcapL2(t, filepath.Join("testdata", "ipv4-ipv4.pcap"))
	bf := sch.NewFlowMessage()
	l := ParseEthernet(sch, bf, pb.RawFlow_DECAP_IPIP, pcap)
	if l != 100 {
		t.Errorf("ParseEthernet() returned %d, expected 100", l)
	}
	expected := &schema.FlowMessage{
		SrcAddr: netip.MustParseAddr("::ffff:192.168.0.1"),
		DstAddr: netip.MustParseAddr("::ffff:192.168.0.2"),
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnEType:        uint32(constants.ETypeIPv4),
			schema.ColumnProto:        uint32(constants.ProtoICMPv4),
			schema.ColumnIPTTL:        uint8(255),
			schema.ColumnIPFragmentID: uint32(18),
			schema.ColumnICMPv4Type:   uint8(8),
		},
	}
	if diff := helpers.Diff(bf, expected); diff != "" {
		t.Fatalf("ParseEthernet() (-got, +want):\n%s", diff)
	}
}

func TestDecodeIPv4IPv6(t *testing.T) {
	sch := schema.NewMock(t).EnableAllColumns()
	pcap := helpers.ReadPcapL2(t, filepath.Join("testdata", "ipv4-ipv6.pcap"))
	bf := sch.NewFlowMessage()
	l := ParseEthernet(sch, bf, pb.RawFlow_DECAP_IPIP, pcap)
	if l != 60 {
		t.Errorf("ParseEthernet() returned %d, expected 60", l)
	}
	expected := &schema.FlowMessage{
		SrcAddr: netip.MustParseAddr("2001:610:1908:a000::149:20"),
		DstAddr: netip.MustParseAddr("2002:2470:9ffa:fa9:0:dd:ed00:2"),
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnEType:    uint32(constants.ETypeIPv6),
			schema.ColumnProto:    uint32(constants.ProtoTCP),
			schema.ColumnIPTTL:    uint8(58),
			schema.ColumnSrcPort:  uint16(80),
			schema.ColumnDstPort:  uint16(35673),
			schema.ColumnTCPFlags: uint16(16),
		},
	}
	if diff := helpers.Diff(bf, expected); diff != "" {
		t.Fatalf("ParseEthernet() (-got, +want):\n%s", diff)
	}
}

func TestDecodeGREv6Plain(t *testing.T) {
	sch := schema.NewMock(t).EnableAllColumns()
	pcap := helpers.ReadPcapL2(t, filepath.Join("testdata", "gre-v6-plain.pcap"))
	bf := sch.NewFlowMessage()
	l := ParseEthernet(sch, bf, pb.RawFlow_DECAP_GRE, pcap)
	if l != 62 {
		t.Errorf("ParseEthernet() returned %d, expected 62", l)
	}
	expected := &schema.FlowMessage{
		SrcAddr: netip.MustParseAddr("2001:db8::2"),
		DstAddr: netip.MustParseAddr("2001:db8::1"),
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnEType:    uint32(constants.ETypeIPv6),
			schema.ColumnProto:    uint32(constants.ProtoTCP),
			schema.ColumnIPTTL:    uint8(255),
			schema.ColumnSrcPort:  uint16(18716),
			schema.ColumnDstPort:  uint16(23),
			schema.ColumnIPTos:    uint8(192),
			schema.ColumnTCPFlags: uint16(24),
		},
	}
	if diff := helpers.Diff(bf, expected); diff != "" {
		t.Fatalf("ParseEthernet() (-got, +want):\n%s", diff)
	}
}

func TestDecodeGREv4Plain(t *testing.T) {
	sch := schema.NewMock(t).EnableAllColumns()
	pcap := helpers.ReadPcapL2(t, filepath.Join("testdata", "gre-v4-plain.pcap"))
	bf := sch.NewFlowMessage()
	l := ParseEthernet(sch, bf, pb.RawFlow_DECAP_GRE, pcap)
	if l != 76 {
		t.Errorf("ParseEthernet() returned %d, expected 76", l)
	}
	expected := &schema.FlowMessage{
		SrcAddr: netip.MustParseAddr("::ffff:66.59.111.190"),
		DstAddr: netip.MustParseAddr("::ffff:66.59.111.182"),
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnEType:   uint32(constants.ETypeIPv4),
			schema.ColumnProto:   uint32(constants.ProtoUDP),
			schema.ColumnIPTTL:   uint8(64),
			schema.ColumnSrcPort: uint16(123),
			schema.ColumnDstPort: uint16(123),
			schema.ColumnIPTos:   uint8(16),
		},
	}
	if diff := helpers.Diff(bf, expected); diff != "" {
		t.Fatalf("ParseEthernet() (-got, +want):\n%s", diff)
	}
}

func TestDecodeGREv4Key(t *testing.T) {
	sch := schema.NewMock(t).EnableAllColumns()
	pcap := helpers.ReadPcapL2(t, filepath.Join("testdata", "gre-v4-key.pcap"))
	bf := sch.NewFlowMessage()
	l := ParseEthernet(sch, bf, pb.RawFlow_DECAP_GRE, pcap)
	if l != 100 {
		t.Errorf("ParseEthernet() returned %d, expected 100", l)
	}
	expected := &schema.FlowMessage{
		SrcAddr: netip.MustParseAddr("::ffff:192.168.1.101"),
		DstAddr: netip.MustParseAddr("::ffff:192.168.2.102"),
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnEType:        uint32(constants.ETypeIPv4),
			schema.ColumnProto:        uint32(constants.ProtoICMPv4),
			schema.ColumnIPTTL:        uint8(254),
			schema.ColumnIPFragmentID: uint32(1119),
			schema.ColumnICMPv4Type:   uint8(8),
		},
	}
	if diff := helpers.Diff(bf, expected); diff != "" {
		t.Fatalf("ParseEthernet() (-got, +want):\n%s", diff)
	}
}

func TestDecodeSRv6(t *testing.T) {
	sch := schema.NewMock(t).EnableAllColumns()
	pcap := helpers.ReadPcapL2(t, filepath.Join("testdata", "srv6-end-dt6.pcap"))
	bf := sch.NewFlowMessage()
	l := ParseEthernet(sch, bf, pb.RawFlow_DECAP_SRV6, pcap)
	if l != 64 {
		t.Errorf("ParseEthernet() returned %d, expected 64", l)
	}
	expected := &schema.FlowMessage{
		SrcAddr: netip.MustParseAddr("1:2:1::1"),
		DstAddr: netip.MustParseAddr("b::2"),
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnEType:         uint32(constants.ETypeIPv6),
			schema.ColumnProto:         uint32(constants.ProtoUDP),
			schema.ColumnIPTTL:         uint8(64),
			schema.ColumnIPv6FlowLabel: uint32(0x0ae0a9),
			schema.ColumnSrcPort:       uint16(50701),
			schema.ColumnDstPort:       uint16(5001),
		},
	}
	if diff := helpers.Diff(bf, expected); diff != "" {
		t.Fatalf("ParseEthernet() (-got, +want):\n%s", diff)
	}
}
