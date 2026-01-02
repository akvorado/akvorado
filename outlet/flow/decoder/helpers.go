// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

import (
	"encoding/binary"
	"math/bits"
	"net/netip"

	"akvorado/common/constants"
	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/schema"
)

// ParseIPv4 parses an IPv4 packet and returns layer-3 length.
func ParseIPv4(sch *schema.Component, bf *schema.FlowMessage, decap pb.RawFlow_DecapsulationProtocol, data []byte) uint64 {
	var l3Length uint64
	var proto uint8
	if len(data) < 20 {
		return 0
	}
	l3Length = uint64(binary.BigEndian.Uint16(data[2:4]))
	fragoffset := binary.BigEndian.Uint16(data[6:8]) & 0x1fff
	proto = data[9]
	if decap == pb.RawFlow_DECAP_NONE {
		bf.AppendUint(schema.ColumnEType, constants.ETypeIPv4)
		bf.SrcAddr = DecodeIP(data[12:16])
		bf.DstAddr = DecodeIP(data[16:20])
		if !sch.IsDisabled(schema.ColumnGroupL3L4) {
			bf.AppendUint(schema.ColumnIPTos, uint64(data[1]))
			bf.AppendUint(schema.ColumnIPTTL, uint64(data[8]))
			bf.AppendUint(schema.ColumnIPFragmentID,
				uint64(binary.BigEndian.Uint16(data[4:6])))
			bf.AppendUint(schema.ColumnIPFragmentOffset,
				uint64(fragoffset))
		}
		bf.AppendUint(schema.ColumnProto, uint64(proto))
	}
	ihl := int((data[0] & 0xf) * 4)
	if len(data) >= ihl {
		data = data[ihl:]
	} else {
		data = data[:0]
	}
	if fragoffset == 0 {
		innerL3Length := ParseL4(sch, bf, decap, data, proto)
		if decap != pb.RawFlow_DECAP_NONE {
			return innerL3Length
		}
		return l3Length
	}
	if decap != pb.RawFlow_DECAP_NONE {
		return 0
	}
	return l3Length
}

// ParseIPv6 parses an IPv6 packet and returns layer-3 length.
func ParseIPv6(sch *schema.Component, bf *schema.FlowMessage, decap pb.RawFlow_DecapsulationProtocol, data []byte) uint64 {
	var l3Length uint64
	var proto uint8
	if len(data) < 40 {
		return 0
	}
	l3Length = uint64(binary.BigEndian.Uint16(data[4:6])) + 40
	proto = data[6]
	if decap == pb.RawFlow_DECAP_NONE {
		bf.AppendUint(schema.ColumnEType, constants.ETypeIPv6)
		bf.SrcAddr = DecodeIP(data[8:24])
		bf.DstAddr = DecodeIP(data[24:40])
		bf.AppendUint(schema.ColumnProto, uint64(proto))
		if !sch.IsDisabled(schema.ColumnGroupL3L4) {
			bf.AppendUint(schema.ColumnIPTos,
				uint64(binary.BigEndian.Uint16(data[0:2])&0xff0>>4))
			bf.AppendUint(schema.ColumnIPTTL, uint64(data[7]))
			bf.AppendUint(schema.ColumnIPv6FlowLabel,
				uint64(binary.BigEndian.Uint32(data[0:4])&0xfffff))
			bf.AppendUint(schema.ColumnProto, uint64(proto))
			// TODO fragmentID/fragmentOffset are in a separate header
		}
	}
	data = data[40:]
	innerL3Length := ParseL4(sch, bf, decap, data, proto)
	if decap != pb.RawFlow_DECAP_NONE {
		return innerL3Length
	}
	return l3Length
}

// ParseL4 parses L4 layer. It returns the L3 length in case there is an encapsulation.
func ParseL4(sch *schema.Component, bf *schema.FlowMessage, decap pb.RawFlow_DecapsulationProtocol, data []byte, proto uint8) uint64 {
	switch decap {
	case pb.RawFlow_DECAP_NONE:
		break
	case pb.RawFlow_DECAP_VXLAN:
		if proto == constants.ProtoUDP && len(data) > 16 && binary.BigEndian.Uint16(data[2:4]) == constants.PortVXLAN {
			// It looks like a VXLAN packet!
			data = data[16:]
			return ParseEthernet(sch, bf, pb.RawFlow_DECAP_NONE, data)
		}
		return 0
	case pb.RawFlow_DECAP_GRE:
		if proto == constants.ProtoGRE && len(data) > 4 {
			flagAndVersion := binary.BigEndian.Uint16(data[:2])
			greProtocol := binary.BigEndian.Uint16(data[2:4])
			// Only handle RFC 2890
			if flagAndVersion&0x4fff != 0 {
				return 0
			}
			skip := 4 + bits.OnesCount16(flagAndVersion)*4
			if len(data) >= skip {
				data = data[skip:]
				switch greProtocol {
				case constants.ETypeIPv4:
					return ParseIPv4(sch, bf, pb.RawFlow_DECAP_NONE, data)
				case constants.ETypeIPv6:
					return ParseIPv6(sch, bf, pb.RawFlow_DECAP_NONE, data)
				}
			}
			return 0
		}
		return 0
	case pb.RawFlow_DECAP_IPIP:
		switch proto {
		case constants.ProtoIPv4:
			return ParseIPv4(sch, bf, pb.RawFlow_DECAP_NONE, data)
		case constants.ProtoIPv6:
			return ParseIPv6(sch, bf, pb.RawFlow_DECAP_NONE, data)
		}
		return 0
	case pb.RawFlow_DECAP_SRV6:
		// An SRv6 packet can looks like than IP in IPv6 packet, with optionally
		// one or several SRH headers. We can handle IPv4 and IPv6 payloads, but
		// not Ethernet ones as there is no hint for that (with DX2, IPv6 next
		// header is "no next header" (59))
		for {
			switch proto {
			case constants.ProtoIPv4:
				return ParseIPv4(sch, bf, pb.RawFlow_DECAP_NONE, data)
			case constants.ProtoIPv6:
				return ParseIPv6(sch, bf, pb.RawFlow_DECAP_NONE, data)
			case constants.ProtoSRH:
				if len(data) < 8 || data[2] != 4 {
					return 0
				}
				skip := 8 + int(data[1])*8
				if len(data) < skip {
					return 0
				}
				proto = data[0]
				data = data[skip:]
			default:
				return 0
			}
		}
	}
	if proto == constants.ProtoTCP || proto == constants.ProtoUDP {
		if len(data) > 4 {
			bf.AppendUint(schema.ColumnSrcPort,
				uint64(binary.BigEndian.Uint16(data[0:2])))
			bf.AppendUint(schema.ColumnDstPort,
				uint64(binary.BigEndian.Uint16(data[2:4])))
		}
	}
	if !sch.IsDisabled(schema.ColumnGroupL3L4) {
		switch proto {
		case constants.ProtoTCP:
			if len(data) > 13 {
				bf.AppendUint(schema.ColumnTCPFlags,
					uint64(data[13]))
			}
		case constants.ProtoICMPv4:
			if len(data) > 2 {
				bf.AppendUint(schema.ColumnICMPv4Type,
					uint64(data[0]))
				bf.AppendUint(schema.ColumnICMPv4Code,
					uint64(data[1]))
			}
		case constants.ProtoICMPv6:
			if len(data) > 2 {
				bf.AppendUint(schema.ColumnICMPv6Type,
					uint64(data[0]))
				bf.AppendUint(schema.ColumnICMPv6Code,
					uint64(data[1]))
			}
		}
	}
	return 0
}

// ParseEthernet parses an Ethernet packet and returns L3 length.
func ParseEthernet(sch *schema.Component, bf *schema.FlowMessage, decap pb.RawFlow_DecapsulationProtocol, data []byte) uint64 {
	if len(data) < 14 {
		return 0
	}
	if !sch.IsDisabled(schema.ColumnGroupL2) && decap == pb.RawFlow_DECAP_NONE {
		bf.AppendUint(schema.ColumnDstMAC,
			binary.BigEndian.Uint64([]byte{0, 0, data[0], data[1], data[2], data[3], data[4], data[5]}))
		bf.AppendUint(schema.ColumnSrcMAC,
			binary.BigEndian.Uint64([]byte{0, 0, data[6], data[7], data[8], data[9], data[10], data[11]}))
	}
	etherType := binary.BigEndian.Uint16(data[12:14])
	data = data[14:]
	var vlan uint16
	for etherType == constants.ETypeVLAN {
		if len(data) < 4 {
			return 0
		}
		if !sch.IsDisabled(schema.ColumnGroupL2) && decap == pb.RawFlow_DECAP_NONE {
			vlan = (uint16(data[0]&0xf) << 8) + uint16(data[1])
		}
		etherType = binary.BigEndian.Uint16(data[2:4])
		data = data[4:]
	}
	if vlan != 0 && bf.SrcVlan == 0 {
		bf.SrcVlan = vlan
	}
	if etherType == constants.ETypeMPLS {
		mplsLabels := make([]uint32, 0, 5)
		for {
			if len(data) < 5 {
				return 0
			}
			label := binary.BigEndian.Uint32(append([]byte{0}, data[:3]...)) >> 4
			bottom := data[2] & 1
			data = data[4:]
			mplsLabels = append(mplsLabels, label)
			if bottom == 1 || label <= 15 {
				switch data[0] & 0xf0 >> 4 {
				case 4:
					etherType = constants.ETypeIPv4
				case 6:
					etherType = constants.ETypeIPv6
				default:
					return 0
				}
				break
			}
		}
		if len(mplsLabels) > 0 && decap == pb.RawFlow_DECAP_NONE {
			bf.AppendArrayUInt32(schema.ColumnMPLSLabels, mplsLabels)
		}
	}
	switch etherType {
	case constants.ETypeIPv4:
		return ParseIPv4(sch, bf, decap, data)
	case constants.ETypeIPv6:
		return ParseIPv6(sch, bf, decap, data)
	}
	return 0
}

// DecodeIP decodes an IP address
func DecodeIP(b []byte) netip.Addr {
	if ip, ok := netip.AddrFromSlice(b); ok {
		return helpers.AddrTo6(ip)
	}
	return netip.Addr{}
}
