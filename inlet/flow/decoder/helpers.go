// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

import (
	"encoding/binary"
	"net/netip"

	"akvorado/common/helpers"
	"akvorado/common/schema"
)

// ParseIPv4 parses an IPv4 packet and returns layer-3 length.
func ParseIPv4(sch *schema.Component, bf *schema.FlowMessage, data []byte) uint64 {
	var l3length uint64
	var proto uint8
	if len(data) < 20 {
		return 0
	}
	sch.ProtobufAppendVarint(bf, schema.ColumnEType, helpers.ETypeIPv4)
	l3length = uint64(binary.BigEndian.Uint16(data[2:4]))
	bf.SrcAddr = DecodeIP(data[12:16])
	bf.DstAddr = DecodeIP(data[16:20])
	proto = data[9]
	fragoffset := binary.BigEndian.Uint16(data[6:8]) & 0x1fff
	if !sch.IsDisabled(schema.ColumnGroupL3L4) {
		sch.ProtobufAppendVarint(bf, schema.ColumnIPTos, uint64(data[1]))
		sch.ProtobufAppendVarint(bf, schema.ColumnIPTTL, uint64(data[8]))
		sch.ProtobufAppendVarint(bf, schema.ColumnIPFragmentID,
			uint64(binary.BigEndian.Uint16(data[4:6])))
		sch.ProtobufAppendVarint(bf, schema.ColumnIPFragmentOffset,
			uint64(fragoffset))
	}
	ihl := int((data[0] & 0xf) * 4)
	if len(data) >= ihl {
		data = data[ihl:]
	} else {
		data = data[:0]
	}
	sch.ProtobufAppendVarint(bf, schema.ColumnProto, uint64(proto))
	if fragoffset == 0 {
		ParseL4(sch, bf, data, proto)
	}
	return l3length
}

// ParseIPv6 parses an IPv6 packet and returns layer-3 length.
func ParseIPv6(sch *schema.Component, bf *schema.FlowMessage, data []byte) uint64 {
	var l3length uint64
	var proto uint8
	if len(data) < 40 {
		return 0
	}
	l3length = uint64(binary.BigEndian.Uint16(data[4:6])) + 40
	sch.ProtobufAppendVarint(bf, schema.ColumnEType, helpers.ETypeIPv6)
	bf.SrcAddr = DecodeIP(data[8:24])
	bf.DstAddr = DecodeIP(data[24:40])
	proto = data[6]
	sch.ProtobufAppendVarint(bf, schema.ColumnProto, uint64(proto))
	if !sch.IsDisabled(schema.ColumnGroupL3L4) {
		sch.ProtobufAppendVarint(bf, schema.ColumnIPTos,
			uint64(binary.BigEndian.Uint16(data[0:2])&0xff0>>4))
		sch.ProtobufAppendVarint(bf, schema.ColumnIPTTL, uint64(data[7]))
		sch.ProtobufAppendVarint(bf, schema.ColumnIPv6FlowLabel,
			uint64(binary.BigEndian.Uint32(data[0:4])&0xfffff))
		// TODO fragmentID/fragmentOffset are in a separate header
	}
	data = data[40:]
	sch.ProtobufAppendVarint(bf, schema.ColumnProto, uint64(proto))
	ParseL4(sch, bf, data, proto)
	return l3length
}

// ParseL4 parses L4 layer.
func ParseL4(sch *schema.Component, bf *schema.FlowMessage, data []byte, proto uint8) {
	if proto == 6 || proto == 17 {
		// UDP or TCP
		if len(data) > 4 {
			sch.ProtobufAppendVarint(bf, schema.ColumnSrcPort,
				uint64(binary.BigEndian.Uint16(data[0:2])))
			sch.ProtobufAppendVarint(bf, schema.ColumnDstPort,
				uint64(binary.BigEndian.Uint16(data[2:4])))
		}
	}
	if !sch.IsDisabled(schema.ColumnGroupL3L4) {
		if proto == 6 {
			// TCP
			if len(data) > 13 {
				sch.ProtobufAppendVarint(bf, schema.ColumnTCPFlags,
					uint64(data[13]))
			}
		} else if proto == 1 {
			// ICMPv4
			if len(data) > 2 {
				sch.ProtobufAppendVarint(bf, schema.ColumnICMPv4Type,
					uint64(data[0]))
				sch.ProtobufAppendVarint(bf, schema.ColumnICMPv4Code,
					uint64(data[1]))
			}
		} else if proto == 58 {
			// ICMPv6
			if len(data) > 2 {
				sch.ProtobufAppendVarint(bf, schema.ColumnICMPv6Type,
					uint64(data[0]))
				sch.ProtobufAppendVarint(bf, schema.ColumnICMPv6Code,
					uint64(data[1]))
			}
		}
	}
}

// ParseEthernet parses an Ethernet packet and returns L3 length.
func ParseEthernet(sch *schema.Component, bf *schema.FlowMessage, data []byte) uint64 {
	if len(data) < 14 {
		return 0
	}
	if !sch.IsDisabled(schema.ColumnGroupL2) {
		sch.ProtobufAppendVarint(bf, schema.ColumnDstMAC,
			binary.BigEndian.Uint64([]byte{0, 0, data[0], data[1], data[2], data[3], data[4], data[5]}))
		sch.ProtobufAppendVarint(bf, schema.ColumnSrcMAC,
			binary.BigEndian.Uint64([]byte{0, 0, data[6], data[7], data[8], data[9], data[10], data[11]}))
	}
	etherType := data[12:14]
	data = data[14:]
	var vlan uint16
	for etherType[0] == 0x81 && etherType[1] == 0x00 {
		// 802.1q
		if len(data) < 4 {
			return 0
		}
		if !sch.IsDisabled(schema.ColumnGroupL2) {
			vlan = (uint16(data[0]&0xf) << 8) + uint16(data[1])
		}
		etherType = data[2:4]
		data = data[4:]
	}
	if vlan != 0 && bf.SrcVlan == 0 {
		bf.SrcVlan = vlan
	}
	if etherType[0] == 0x88 && etherType[1] == 0x47 {
		// MPLS
		for {
			if len(data) < 5 {
				return 0
			}
			label := binary.BigEndian.Uint32(append([]byte{0}, data[:3]...)) >> 4
			bottom := data[2] & 1
			data = data[4:]
			sch.ProtobufAppendVarint(bf, schema.ColumnMPLSLabels, uint64(label))
			if bottom == 1 || label <= 15 {
				if data[0]&0xf0>>4 == 4 {
					etherType = []byte{0x8, 0x0}
				} else if data[0]&0xf0>>4 == 6 {
					etherType = []byte{0x86, 0xdd}
				} else {
					return 0
				}
				break
			}
		}
	}
	if etherType[0] == 0x8 && etherType[1] == 0x0 {
		return ParseIPv4(sch, bf, data)
	} else if etherType[0] == 0x86 && etherType[1] == 0xdd {
		return ParseIPv6(sch, bf, data)
	}
	return 0
}

// DecodeIP decodes an IP address
func DecodeIP(b []byte) netip.Addr {
	if ip, ok := netip.AddrFromSlice(b); ok {
		return netip.AddrFrom16(ip.As16())
	}
	return netip.Addr{}
}
