// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-FileCopyrightText: 2021 NetSampler
// SPDX-License-Identifier: AGPL-3.0-only AND BSD-3-Clause

package netflow

import (
	"encoding/binary"
	"net/netip"

	"akvorado/common/helpers"
	"akvorado/common/schema"

	"github.com/netsampler/goflow2/decoders/netflow"
	"github.com/netsampler/goflow2/producer"
)

func (nd *Decoder) decode(msgDec interface{}, samplingRateSys producer.SamplingRateSystem) []*schema.FlowMessage {
	flowMessageSet := []*schema.FlowMessage{}
	var obsDomainID uint32
	var dataFlowSet []netflow.DataFlowSet
	var optionsDataFlowSet []netflow.OptionsDataFlowSet
	switch msgDecConv := msgDec.(type) {
	case netflow.NFv9Packet:
		dataFlowSet, _, _, optionsDataFlowSet = producer.SplitNetFlowSets(msgDecConv)
		obsDomainID = msgDecConv.SourceId
	case netflow.IPFIXPacket:
		dataFlowSet, _, _, optionsDataFlowSet = producer.SplitIPFIXSets(msgDecConv)
		obsDomainID = msgDecConv.ObservationDomainId
	default:
		return nil
	}

	// Get sampling rate
	samplingRate, found := producer.SearchNetFlowOptionDataSets(optionsDataFlowSet)
	if samplingRateSys != nil {
		if found {
			samplingRateSys.AddSamplingRate(10, obsDomainID, samplingRate)
		} else {
			samplingRate, _ = samplingRateSys.GetSamplingRate(10, obsDomainID)
		}
	}

	// Parse fields
	for _, dataFlowSetItem := range dataFlowSet {
		for _, record := range dataFlowSetItem.Records {
			flow := nd.decodeRecord(record.Values)
			if flow != nil {
				flow.SamplingRate = samplingRate
				flowMessageSet = append(flowMessageSet, flow)
			}
		}
	}

	return flowMessageSet
}

func (nd *Decoder) decodeRecord(fields []netflow.DataField) *schema.FlowMessage {
	var etype uint16
	bf := &schema.FlowMessage{}
	for _, field := range fields {
		v, ok := field.Value.([]byte)
		if !ok {
			continue
		}
		if field.PenProvided {
			continue
		}

		switch field.Type {
		// Statistics
		case netflow.NFV9_FIELD_IN_BYTES, netflow.NFV9_FIELD_OUT_BYTES:
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnBytes, decodeUNumber(v))
		case netflow.NFV9_FIELD_IN_PKTS, netflow.NFV9_FIELD_OUT_PKTS:
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnPackets, decodeUNumber(v))

		// L3
		case netflow.NFV9_FIELD_IPV4_SRC_ADDR:
			etype = helpers.ETypeIPv4
			bf.SrcAddr = decodeIP(v)
		case netflow.NFV9_FIELD_IPV4_DST_ADDR:
			etype = helpers.ETypeIPv4
			bf.DstAddr = decodeIP(v)
		case netflow.NFV9_FIELD_IPV6_SRC_ADDR:
			etype = helpers.ETypeIPv6
			bf.SrcAddr = decodeIP(v)
		case netflow.NFV9_FIELD_IPV6_DST_ADDR:
			etype = helpers.ETypeIPv6
			bf.DstAddr = decodeIP(v)
		case netflow.NFV9_FIELD_SRC_MASK, netflow.NFV9_FIELD_IPV6_SRC_MASK:
			bf.SrcNetMask = uint8(decodeUNumber(v))
		case netflow.NFV9_FIELD_DST_MASK, netflow.NFV9_FIELD_IPV6_DST_MASK:
			bf.DstNetMask = uint8(decodeUNumber(v))
		case netflow.NFV9_FIELD_IPV4_NEXT_HOP, netflow.NFV9_FIELD_BGP_IPV4_NEXT_HOP, netflow.NFV9_FIELD_IPV6_NEXT_HOP, netflow.NFV9_FIELD_BGP_IPV6_NEXT_HOP:
			bf.NextHop = decodeIP(v)

		// L4
		case netflow.NFV9_FIELD_L4_SRC_PORT:
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnSrcPort, decodeUNumber(v))
		case netflow.NFV9_FIELD_L4_DST_PORT:
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnDstPort, decodeUNumber(v))
		case netflow.NFV9_FIELD_PROTOCOL:
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnProto, decodeUNumber(v))

		// Network
		case netflow.NFV9_FIELD_SRC_AS:
			bf.SrcAS = uint32(decodeUNumber(v))
		case netflow.NFV9_FIELD_DST_AS:
			bf.DstAS = uint32(decodeUNumber(v))

		// Interfaces
		case netflow.NFV9_FIELD_INPUT_SNMP:
			bf.InIf = uint32(decodeUNumber(v))
		case netflow.NFV9_FIELD_OUTPUT_SNMP:
			bf.OutIf = uint32(decodeUNumber(v))

		// Remaining
		case netflow.NFV9_FIELD_FORWARDING_STATUS:
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnForwardingStatus, decodeUNumber(v))
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
				case netflow.NFV9_FIELD_SRC_VLAN:
					bf.SrcVlan = uint16(decodeUNumber(v))
				case netflow.NFV9_FIELD_DST_VLAN:
					bf.DstVlan = uint16(decodeUNumber(v))
				case netflow.NFV9_FIELD_IN_SRC_MAC:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnSrcMAC, decodeUNumber(v))
				case netflow.NFV9_FIELD_IN_DST_MAC:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnDstMAC, decodeUNumber(v))
				case netflow.NFV9_FIELD_OUT_SRC_MAC:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnSrcMAC, decodeUNumber(v))
				case netflow.NFV9_FIELD_OUT_DST_MAC:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnDstMAC, decodeUNumber(v))
				}
			}

			if !nd.d.Schema.IsDisabled(schema.ColumnGroupL3L4) {
				// Misc L3/L4 fields
				switch field.Type {
				case netflow.NFV9_FIELD_MIN_TTL:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPTTL, decodeUNumber(v))
				case netflow.NFV9_FIELD_SRC_TOS:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPTos, decodeUNumber(v))
				case netflow.NFV9_FIELD_IPV6_FLOW_LABEL:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPv6FlowLabel, decodeUNumber(v))
				case netflow.NFV9_FIELD_TCP_FLAGS:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnTCPFlags, decodeUNumber(v))
				case netflow.NFV9_FIELD_IPV4_IDENT:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPFragmentID, decodeUNumber(v))
				case netflow.NFV9_FIELD_FRAGMENT_OFFSET:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPFragmentOffset, decodeUNumber(v))

				// ICMP
				case netflow.NFV9_FIELD_ICMP_TYPE, netflow.IPFIX_FIELD_icmpTypeCodeIPv6:
					icmpTypeCode := decodeUNumber(v)
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnICMPType, icmpTypeCode>>8)
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnICMPCode, icmpTypeCode&0xff)
				case netflow.IPFIX_FIELD_icmpTypeIPv4, netflow.IPFIX_FIELD_icmpTypeIPv6:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnICMPType, decodeUNumber(v))
				case netflow.IPFIX_FIELD_icmpCodeIPv4, netflow.IPFIX_FIELD_icmpCodeIPv6:
					nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnICMPCode, decodeUNumber(v))
				}
			}
		}
	}
	nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnEType, uint64(etype))
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
