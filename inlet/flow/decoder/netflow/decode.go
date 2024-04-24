// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-FileCopyrightText: 2021 NetSampler
// SPDX-License-Identifier: AGPL-3.0-only AND BSD-3-Clause

package netflow

import (
	"encoding/binary"
	"net/netip"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/inlet/flow/decoder"

	"github.com/netsampler/goflow2/v2/decoders/netflow"
)

// When decoding, we use IPFIX information element identifiers. However, it
// should be noted, as per RFC 5102, IPFIX "Information Element identifier
// values in the sub-range of 1-127 are compatible with field types used by
// NetFlow version 9 [RFC3954]."

func (nd *Decoder) decodeIPFIX(packet netflow.IPFIXPacket, samplingRateSys *samplingRateSystem, sysOffset uint64) []*schema.FlowMessage {
	obsDomainID := packet.ObservationDomainId
	return nd.decodeCommon(10, obsDomainID, packet.FlowSets, samplingRateSys, sysOffset)
}

func (nd *Decoder) decodeNFv9(packet netflow.NFv9Packet, samplingRateSys *samplingRateSystem, sysOffset uint64) []*schema.FlowMessage {
	obsDomainID := packet.SourceId
	return nd.decodeCommon(9, obsDomainID, packet.FlowSets, samplingRateSys, sysOffset)
}

func (nd *Decoder) decodeCommon(version uint16, obsDomainID uint32, flowSets []interface{}, samplingRateSys *samplingRateSystem, sysOffset uint64) []*schema.FlowMessage {
	flowMessageSet := []*schema.FlowMessage{}

	// Look for sampling rate in option data flowsets
	for _, flowSet := range flowSets {
		switch tFlowSet := flowSet.(type) {
		case netflow.OptionsDataFlowSet:
			for _, record := range tFlowSet.Records {
				var (
					samplingRate                uint32
					samplerID                   uint64
					packetInterval, packetSpace uint32
				)
				for _, field := range record.OptionsValues {
					v, ok := field.Value.([]byte)
					if !ok {
						continue
					}
					if field.PenProvided {
						continue
					}
					switch field.Type {
					case netflow.IPFIX_FIELD_samplingInterval, netflow.IPFIX_FIELD_samplerRandomInterval:
						samplingRate = uint32(decodeUNumber(v))
					case netflow.IPFIX_FIELD_samplerId, netflow.IPFIX_FIELD_selectorId:
						samplerID = uint64(decodeUNumber(v))
					case netflow.IPFIX_FIELD_samplingPacketInterval:
						packetInterval = uint32(decodeUNumber(v))
					case netflow.IPFIX_FIELD_samplingPacketSpace:
						packetSpace = uint32(decodeUNumber(v))
					}
				}
				if packetInterval > 0 {
					samplingRate = (packetInterval + packetSpace) / packetInterval
				}
				if samplingRate > 0 {
					samplingRateSys.SetSamplingRate(version, obsDomainID, samplerID, samplingRate)
				}
			}
		case netflow.DataFlowSet:
			for _, record := range tFlowSet.Records {
				flow := nd.decodeRecord(version, obsDomainID, samplingRateSys, record.Values, sysOffset)
				if flow != nil {
					flowMessageSet = append(flowMessageSet, flow)
				}
			}
		}
	}

	return flowMessageSet
}

func (nd *Decoder) decodeRecord(version uint16, obsDomainID uint32, samplingRateSys *samplingRateSystem, fields []netflow.DataField, sysOffset uint64) *schema.FlowMessage {
	var etype, dstPort, srcPort uint16
	var proto, icmpType, icmpCode uint8
	var foundIcmpTypeCode bool
	bf := &schema.FlowMessage{}
	dataLinkFrameSectionIdx := -1
	for idx, field := range fields {
		v, ok := field.Value.([]byte)
		if !ok {
			continue
		}
		if field.PenProvided {
			continue
		}

		switch field.Type {
		// Statistics
		case netflow.IPFIX_FIELD_octetDeltaCount, netflow.IPFIX_FIELD_postOctetDeltaCount, netflow.IPFIX_FIELD_initiatorOctets, netflow.IPFIX_FIELD_responderOctets:
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnBytes, decodeUNumber(v))
		case netflow.IPFIX_FIELD_packetDeltaCount, netflow.IPFIX_FIELD_postPacketDeltaCount:
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnPackets, decodeUNumber(v))
		case netflow.IPFIX_FIELD_samplingInterval, netflow.IPFIX_FIELD_samplerRandomInterval:
			bf.SamplingRate = uint32(decodeUNumber(v))
		case netflow.IPFIX_FIELD_samplerId, netflow.IPFIX_FIELD_selectorId:
			bf.SamplingRate = samplingRateSys.GetSamplingRate(version, obsDomainID, decodeUNumber(v))

		// L3
		case netflow.IPFIX_FIELD_sourceIPv4Address:
			etype = helpers.ETypeIPv4
			bf.SrcAddr = decodeIP(v)
		case netflow.IPFIX_FIELD_destinationIPv4Address:
			etype = helpers.ETypeIPv4
			bf.DstAddr = decodeIP(v)
		case netflow.IPFIX_FIELD_sourceIPv6Address:
			etype = helpers.ETypeIPv6
			bf.SrcAddr = decodeIP(v)
		case netflow.IPFIX_FIELD_destinationIPv6Address:
			etype = helpers.ETypeIPv6
			bf.DstAddr = decodeIP(v)
		case netflow.IPFIX_FIELD_sourceIPv4PrefixLength, netflow.IPFIX_FIELD_sourceIPv6PrefixLength:
			bf.SrcNetMask = uint8(decodeUNumber(v))
		case netflow.IPFIX_FIELD_destinationIPv4PrefixLength, netflow.IPFIX_FIELD_destinationIPv6PrefixLength:
			bf.DstNetMask = uint8(decodeUNumber(v))
		case netflow.IPFIX_FIELD_ipNextHopIPv4Address, netflow.IPFIX_FIELD_bgpNextHopIPv4Address, netflow.IPFIX_FIELD_ipNextHopIPv6Address, netflow.IPFIX_FIELD_bgpNextHopIPv6Address:
			bf.NextHop = decodeIP(v)

		// L4
		case netflow.IPFIX_FIELD_sourceTransportPort:
			srcPort = uint16(decodeUNumber(v))
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnSrcPort, uint64(srcPort))
		case netflow.IPFIX_FIELD_destinationTransportPort:
			dstPort = uint16(decodeUNumber(v))
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnDstPort, uint64(dstPort))
		case netflow.IPFIX_FIELD_protocolIdentifier:
			proto = uint8(decodeUNumber(v))
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnProto, uint64(proto))

		// Network
		case netflow.IPFIX_FIELD_bgpSourceAsNumber:
			bf.SrcAS = uint32(decodeUNumber(v))
		case netflow.IPFIX_FIELD_bgpDestinationAsNumber:
			bf.DstAS = uint32(decodeUNumber(v))

		// Interfaces
		case netflow.IPFIX_FIELD_ingressInterface:
			bf.InIf = uint32(decodeUNumber(v))
		case netflow.IPFIX_FIELD_egressInterface:
			bf.OutIf = uint32(decodeUNumber(v))

		// RFC7133: process it later to not override other fields
		case netflow.IPFIX_FIELD_dataLinkFrameSize:
			// We are going to ignore it as we don't know L3 size yet.
		case netflow.IPFIX_FIELD_dataLinkFrameSection:
			dataLinkFrameSectionIdx = idx

		// MPLS
		case netflow.IPFIX_FIELD_mplsTopLabelStackSection, netflow.IPFIX_FIELD_mplsLabelStackSection2, netflow.IPFIX_FIELD_mplsLabelStackSection3, netflow.IPFIX_FIELD_mplsLabelStackSection4, netflow.IPFIX_FIELD_mplsLabelStackSection5, netflow.IPFIX_FIELD_mplsLabelStackSection6, netflow.IPFIX_FIELD_mplsLabelStackSection7, netflow.IPFIX_FIELD_mplsLabelStackSection8, netflow.IPFIX_FIELD_mplsLabelStackSection9, netflow.IPFIX_FIELD_mplsLabelStackSection10:
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnMPLSLabels, decodeUNumber(v)>>4)

		// Remaining
		case netflow.IPFIX_FIELD_forwardingStatus:
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnForwardingStatus, decodeUNumber(v))
		case netflow.IPFIX_FIELD_flowStartSysUpTime:
			bf.TimeReceived = decodeUNumber(v) + sysOffset
		default:

			if !nd.d.Schema.IsDisabled(schema.ColumnGroupNAT) {
				// NAT
				switch field.Type {
				case netflow.IPFIX_FIELD_postNATSourceIPv4Address:
					nd.d.Schema.ProtobufAppendIP(bf, schema.ColumnSrcAddrNAT, decodeIP(v))
				case netflow.IPFIX_FIELD_postNATDestinationIPv4Address:
					nd.d.Schema.ProtobufAppendIP(bf, schema.ColumnDstAddrNAT, decodeIP(v))
				case netflow.IPFIX_FIELD_postNAPTSourceTransportPort:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnSrcPortNAT, decodeUNumber(v))
				case netflow.IPFIX_FIELD_postNAPTDestinationTransportPort:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnDstPortNAT, decodeUNumber(v))
				}
			}

			if !nd.d.Schema.IsDisabled(schema.ColumnGroupL2) {
				// L2
				switch field.Type {
				case netflow.IPFIX_FIELD_vlanId:
					bf.SrcVlan = uint16(decodeUNumber(v))
				case netflow.IPFIX_FIELD_postVlanId:
					bf.DstVlan = uint16(decodeUNumber(v))
				case netflow.IPFIX_FIELD_sourceMacAddress:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnSrcMAC, decodeUNumber(v))
				case netflow.IPFIX_FIELD_destinationMacAddress:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnDstMAC, decodeUNumber(v))
				case netflow.IPFIX_FIELD_postSourceMacAddress:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnSrcMAC, decodeUNumber(v))
				case netflow.IPFIX_FIELD_postDestinationMacAddress:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnDstMAC, decodeUNumber(v))
				}
			}

			if !nd.d.Schema.IsDisabled(schema.ColumnGroupL3L4) {
				// Misc L3/L4 fields
				switch field.Type {
				case netflow.IPFIX_FIELD_minimumTTL:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPTTL, decodeUNumber(v))
				case netflow.IPFIX_FIELD_ipClassOfService:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPTos, decodeUNumber(v))
				case netflow.IPFIX_FIELD_flowLabelIPv6:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPv6FlowLabel, decodeUNumber(v))
				case netflow.IPFIX_FIELD_tcpControlBits:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnTCPFlags, decodeUNumber(v))
				case netflow.IPFIX_FIELD_fragmentIdentification:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPFragmentID, decodeUNumber(v))
				case netflow.IPFIX_FIELD_fragmentOffset:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPFragmentOffset, decodeUNumber(v))

				// ICMP
				case netflow.IPFIX_FIELD_icmpTypeCodeIPv4, netflow.IPFIX_FIELD_icmpTypeCodeIPv6:
					icmpTypeCode := decodeUNumber(v)
					icmpType = uint8(icmpTypeCode >> 8)
					icmpCode = uint8(icmpTypeCode & 0xff)
					foundIcmpTypeCode = true
				case netflow.IPFIX_FIELD_icmpTypeIPv4, netflow.IPFIX_FIELD_icmpTypeIPv6:
					icmpType = uint8(decodeUNumber(v))
					foundIcmpTypeCode = true
				case netflow.IPFIX_FIELD_icmpCodeIPv4, netflow.IPFIX_FIELD_icmpCodeIPv6:
					icmpCode = uint8(decodeUNumber(v))
					foundIcmpTypeCode = true
				}
			}
		}
	}
	if dataLinkFrameSectionIdx >= 0 {
		data := fields[dataLinkFrameSectionIdx].Value.([]byte)
		if l3Length := decoder.ParseEthernet(nd.d.Schema, bf, data); l3Length > 0 {
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnBytes, l3Length)
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnPackets, 1)
		}
	}
	if !nd.d.Schema.IsDisabled(schema.ColumnGroupL3L4) && (proto == 1 || proto == 58) {
		// ICMP
		if !foundIcmpTypeCode {
			// Some implementations may use source and destination ports, some
			// other only destination port. The following heuristic is safe as
			// the only valid code for type 0 is 0 (echo reply).
			if srcPort == 0 {
				// Use destination port instead (Cisco on NFv5 that still exists
				// today with NFv9 and IPFIX).
				icmpType = uint8(dstPort >> 8)
				icmpCode = uint8(dstPort & 0xff)
			} else {
				icmpType = uint8(srcPort)
				icmpType = uint8(dstPort)
			}
		}
		if proto == 1 {
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnICMPv4Type, uint64(icmpType))
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnICMPv4Code, uint64(icmpCode))
		} else {
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnICMPv6Type, uint64(icmpType))
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnICMPv6Code, uint64(icmpCode))
		}
	}
	nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnEType, uint64(etype))
	if bf.SamplingRate == 0 {
		bf.SamplingRate = samplingRateSys.GetSamplingRate(version, obsDomainID, 0)
	}
	return bf
}

func decodeUNumber(b []byte) uint64 {
	var o uint64
	l := len(b)
	switch l {
	case 1:
		o = uint64(b[0])
	case 2:
		o = uint64(binary.BigEndian.Uint16(b))
	case 4:
		o = uint64(binary.BigEndian.Uint32(b))
	case 8:
		o = binary.BigEndian.Uint64(b)
	default:
		if l < 8 {
			var iter uint
			for i := range b {
				o |= uint64(b[i]) << uint(8*(uint(l)-iter-1))
				iter++
			}
		} else {
			return 0
		}
	}
	return o
}

func decodeIP(b []byte) netip.Addr {
	if ip, ok := netip.AddrFromSlice(b); ok {
		return netip.AddrFrom16(ip.As16())
	}
	return netip.Addr{}
}
