// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

import (
	"net/netip"
	"path/filepath"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/schema"
)

func TestDecodeMPLSAndIPv4(t *testing.T) {
	sch := schema.NewMock(t).EnableAllColumns()
	pcap := helpers.ReadPcapL2(t, filepath.Join("testdata", "mpls-ipv4.pcap"))
	bf := sch.NewFlowMessage()
	l := ParseEthernet(sch, bf, pcap)
	if l != 40 {
		t.Errorf("ParseEthernet() returned %d, expected 40", l)
	}
	expected := &schema.FlowMessage{
		SrcAddr: netip.MustParseAddr("::ffff:10.31.0.1"),
		DstAddr: netip.MustParseAddr("::ffff:10.34.0.1"),
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnEType:        uint32(helpers.ETypeIPv4),
			schema.ColumnProto:        uint32(6),
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
	l := ParseEthernet(sch, bf, pcap)
	if l != 179 {
		t.Errorf("ParseEthernet() returned %d, expected 179", l)
	}
	expected := &schema.FlowMessage{
		SrcVlan: 100,
		SrcAddr: netip.MustParseAddr("2402:f000:1:8e01::5555"),
		DstAddr: netip.MustParseAddr("2607:fcd0:100:2300::b108:2a6b"),
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnEType:  uint32(helpers.ETypeIPv6),
			schema.ColumnProto:  uint32(4),
			schema.ColumnIPTTL:  uint8(246),
			schema.ColumnSrcMAC: uint64(0x00121ef2613d),
			schema.ColumnDstMAC: uint64(0xc500000082c4),
		},
	}
	if diff := helpers.Diff(bf, expected); diff != "" {
		t.Fatalf("ParseEthernet() (-got, +want):\n%s", diff)
	}
}
