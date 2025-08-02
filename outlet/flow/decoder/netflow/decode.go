// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-FileCopyrightText: 2021 NetSampler
// SPDX-License-Identifier: AGPL-3.0-only AND BSD-3-Clause

package netflow

import (
	"encoding/binary"
	"net/netip"

	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"

	"github.com/netsampler/goflow2/v2/decoders/netflow"
	"github.com/netsampler/goflow2/v2/decoders/netflowlegacy"
)

// When decoding, we use IPFIX information element identifiers. However, it
// should be noted, as per RFC 5102, IPFIX "Information Element identifier
// values in the sub-range of 1-127 are compatible with field types used by
// NetFlow version 9 [RFC3954]."

func (nd *Decoder) decodeNFv5(packet *netflowlegacy.PacketNetFlowV5, ts, sysUptime uint64, options decoder.Option, bf *schema.FlowMessage, finalize decoder.FinalizeFlowFunc) {
	for _, record := range packet.Records {
		bf.SamplingRate = uint64(packet.SamplingInterval)
		bf.InIf = uint32(record.Input)
		bf.OutIf = uint32(record.Output)
		bf.SrcAddr = decodeIPFromUint32(uint32(record.SrcAddr))
		bf.DstAddr = decodeIPFromUint32(uint32(record.DstAddr))
		bf.NextHop = decodeIPFromUint32(uint32(record.NextHop))
		bf.SrcNetMask = record.SrcMask
		bf.DstNetMask = record.DstMask
		bf.SrcAS = uint32(record.SrcAS)
		bf.DstAS = uint32(record.DstAS)
		bf.AppendUint(schema.ColumnBytes, uint64(record.DOctets))
		bf.AppendUint(schema.ColumnPackets, uint64(record.DPkts))
		bf.AppendUint(schema.ColumnEType, helpers.ETypeIPv4)
		bf.AppendUint(schema.ColumnProto, uint64(record.Proto))
		bf.AppendUint(schema.ColumnSrcPort, uint64(record.SrcPort))
		bf.AppendUint(schema.ColumnDstPort, uint64(record.DstPort))
		if !nd.d.Schema.IsDisabled(schema.ColumnGroupL3L4) {
			bf.AppendUint(schema.ColumnIPTos, uint64(record.Tos))
			bf.AppendUint(schema.ColumnTCPFlags, uint64(record.TCPFlags))
		}
		if options.TimestampSource == pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED {
			bf.TimeReceived = uint32(ts - sysUptime + uint64(record.First))
		}
		if bf.SamplingRate == 0 {
			bf.SamplingRate = 1
		}
		finalize()
	}
}

func (nd *Decoder) decodeNFv9IPFIX(version uint16, obsDomainID uint32, flowSets []any, samplingRateSys *samplingRateSystem, ts, sysUptime uint64, options decoder.Option, bf *schema.FlowMessage, finalize decoder.FinalizeFlowFunc) {
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
					if !ok || field.PenProvided {
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
				nd.decodeRecord(version, obsDomainID, samplingRateSys, record.Values, ts, sysUptime, options, bf)
				finalize()
			}
		}
	}
}

func (nd *Decoder) decodeRecord(version uint16, obsDomainID uint32, samplingRateSys *samplingRateSystem, fields []netflow.DataField, ts, sysUptime uint64, options decoder.Option, bf *schema.FlowMessage) {
	var etype, dstPort, srcPort uint16
	var proto, icmpType, icmpCode uint8
	var foundIcmpTypeCode bool
	mplsLabels := make([]uint32, 0, 5)
	dataLinkFrameSectionIdx := -1
	for idx, field := range fields {
		v, ok := field.Value.([]byte)
		if !ok || field.PenProvided {
			continue
		}

		switch field.Type {
		// Statistics
		case netflow.IPFIX_FIELD_octetDeltaCount, netflow.IPFIX_FIELD_postOctetDeltaCount, netflow.IPFIX_FIELD_initiatorOctets, netflow.IPFIX_FIELD_responderOctets:
			bf.AppendUint(schema.ColumnBytes, decodeUNumber(v))
		case netflow.IPFIX_FIELD_packetDeltaCount, netflow.IPFIX_FIELD_postPacketDeltaCount:
			bf.AppendUint(schema.ColumnPackets, decodeUNumber(v))
		case netflow.IPFIX_FIELD_samplingInterval, netflow.IPFIX_FIELD_samplerRandomInterval:
			bf.SamplingRate = decodeUNumber(v)
		case netflow.IPFIX_FIELD_samplerId, netflow.IPFIX_FIELD_selectorId:
			bf.SamplingRate = uint64(samplingRateSys.GetSamplingRate(version, obsDomainID, decodeUNumber(v)))

		// L3
		case netflow.IPFIX_FIELD_sourceIPv4Address:
			etype = helpers.ETypeIPv4
			bf.SrcAddr = decodeIPFromBytes(v)
		case netflow.IPFIX_FIELD_destinationIPv4Address:
			etype = helpers.ETypeIPv4
			bf.DstAddr = decodeIPFromBytes(v)
		case netflow.IPFIX_FIELD_sourceIPv6Address:
			etype = helpers.ETypeIPv6
			bf.SrcAddr = decodeIPFromBytes(v)
		case netflow.IPFIX_FIELD_destinationIPv6Address:
			etype = helpers.ETypeIPv6
			bf.DstAddr = decodeIPFromBytes(v)
		case netflow.IPFIX_FIELD_sourceIPv4PrefixLength, netflow.IPFIX_FIELD_sourceIPv6PrefixLength:
			bf.SrcNetMask = uint8(decodeUNumber(v))
		case netflow.IPFIX_FIELD_destinationIPv4PrefixLength, netflow.IPFIX_FIELD_destinationIPv6PrefixLength:
			bf.DstNetMask = uint8(decodeUNumber(v))
		case netflow.IPFIX_FIELD_ipNextHopIPv4Address, netflow.IPFIX_FIELD_bgpNextHopIPv4Address, netflow.IPFIX_FIELD_ipNextHopIPv6Address, netflow.IPFIX_FIELD_bgpNextHopIPv6Address:
			bf.NextHop = decodeIPFromBytes(v)

		// L4
		case netflow.IPFIX_FIELD_sourceTransportPort:
			srcPort = uint16(decodeUNumber(v))
			bf.AppendUint(schema.ColumnSrcPort, uint64(srcPort))
		case netflow.IPFIX_FIELD_destinationTransportPort:
			dstPort = uint16(decodeUNumber(v))
			bf.AppendUint(schema.ColumnDstPort, uint64(dstPort))
		case netflow.IPFIX_FIELD_protocolIdentifier:
			proto = uint8(decodeUNumber(v))
			bf.AppendUint(schema.ColumnProto, uint64(proto))

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
			uv := decodeUNumber(v) >> 4
			if uv > 0 {
				mplsLabels = append(mplsLabels, uint32(uv))
			}

		// Remaining
		case netflow.IPFIX_FIELD_forwardingStatus:
			bf.AppendUint(schema.ColumnForwardingStatus, decodeUNumber(v))
		default:
			if options.TimestampSource == pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED {
				switch field.Type {
				case netflow.NFV9_FIELD_FIRST_SWITCHED:
					bf.TimeReceived = uint32(ts - sysUptime + decodeUNumber(v))
				case netflow.IPFIX_FIELD_flowStartSeconds:
					bf.TimeReceived = uint32(decodeUNumber(v))
				case netflow.IPFIX_FIELD_flowStartMilliseconds:
					bf.TimeReceived = uint32(decodeUNumber(v) / 1000)
				case netflow.IPFIX_FIELD_flowStartMicroseconds:
					bf.TimeReceived = uint32(decodeUNumber(v) / 1_000_000)
				case netflow.IPFIX_FIELD_flowStartNanoseconds:
					bf.TimeReceived = uint32(ts + decodeUNumber(v)/1_000_000_000)
				}
			}

			if !nd.d.Schema.IsDisabled(schema.ColumnGroupNAT) {
				// NAT
				switch field.Type {
				case netflow.IPFIX_FIELD_postNATSourceIPv4Address:
					bf.AppendIPv6(schema.ColumnSrcAddrNAT, decodeIPFromBytes(v))
				case netflow.IPFIX_FIELD_postNATDestinationIPv4Address:
					bf.AppendIPv6(schema.ColumnDstAddrNAT, decodeIPFromBytes(v))
				case netflow.IPFIX_FIELD_postNAPTSourceTransportPort:
					bf.AppendUint(schema.ColumnSrcPortNAT, decodeUNumber(v))
				case netflow.IPFIX_FIELD_postNAPTDestinationTransportPort:
					bf.AppendUint(schema.ColumnDstPortNAT, decodeUNumber(v))
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
					bf.AppendUint(schema.ColumnSrcMAC, decodeUNumber(v))
				case netflow.IPFIX_FIELD_destinationMacAddress:
					bf.AppendUint(schema.ColumnDstMAC, decodeUNumber(v))
				case netflow.IPFIX_FIELD_postSourceMacAddress:
					bf.AppendUint(schema.ColumnSrcMAC, decodeUNumber(v))
				case netflow.IPFIX_FIELD_postDestinationMacAddress:
					bf.AppendUint(schema.ColumnDstMAC, decodeUNumber(v))
				}
			}

			if !nd.d.Schema.IsDisabled(schema.ColumnGroupL3L4) {
				// Misc L3/L4 fields
				switch field.Type {
				case netflow.IPFIX_FIELD_minimumTTL:
					bf.AppendUint(schema.ColumnIPTTL, decodeUNumber(v))
				case netflow.IPFIX_FIELD_ipClassOfService:
					bf.AppendUint(schema.ColumnIPTos, decodeUNumber(v))
				case netflow.IPFIX_FIELD_flowLabelIPv6:
					bf.AppendUint(schema.ColumnIPv6FlowLabel, decodeUNumber(v))
				case netflow.IPFIX_FIELD_tcpControlBits:
					bf.AppendUint(schema.ColumnTCPFlags, decodeUNumber(v))
				case netflow.IPFIX_FIELD_fragmentIdentification:
					bf.AppendUint(schema.ColumnIPFragmentID, decodeUNumber(v))
				case netflow.IPFIX_FIELD_fragmentOffset:
					bf.AppendUint(schema.ColumnIPFragmentOffset, decodeUNumber(v))

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
			bf.AppendUint(schema.ColumnBytes, l3Length)
			bf.AppendUint(schema.ColumnPackets, 1)
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
			}
			// Unsure how to do the mapping when using source and destination
			// port. Let's ignore.
		}
		if proto == 1 {
			bf.AppendUint(schema.ColumnICMPv4Type, uint64(icmpType))
			bf.AppendUint(schema.ColumnICMPv4Code, uint64(icmpCode))
		} else {
			bf.AppendUint(schema.ColumnICMPv6Type, uint64(icmpType))
			bf.AppendUint(schema.ColumnICMPv6Code, uint64(icmpCode))
		}
	}
	bf.AppendUint(schema.ColumnEType, uint64(etype))
	if len(mplsLabels) > 0 {
		bf.AppendArrayUInt32(schema.ColumnMPLSLabels, mplsLabels)
	}
	if bf.SamplingRate == 0 {
		bf.SamplingRate = uint64(samplingRateSys.GetSamplingRate(version, obsDomainID, 0))
	}
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

func decodeIPFromBytes(b []byte) netip.Addr {
	if ip, ok := netip.AddrFromSlice(b); ok {
		return netip.AddrFrom16(ip.As16())
	}
	return netip.Addr{}
}

func decodeIPFromUint32(ipv4 uint32) netip.Addr {
	var ipBytes [4]byte
	binary.BigEndian.PutUint32(ipBytes[:], ipv4)
	return netip.AddrFrom16(netip.AddrFrom4(ipBytes).As16())
}
