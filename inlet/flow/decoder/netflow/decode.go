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

func decode(msgDec interface{}, samplingRateSys producer.SamplingRateSystem) []*schema.FlowMessage {
	flowMessageSet := []*schema.FlowMessage{}
	var obsDomainID uint32
	var dataFlowSet []netflow.DataFlowSet
	var optionsDataFlowSet []netflow.OptionsDataFlowSet
	var version int
	switch msgDecConv := msgDec.(type) {
	case netflow.NFv9Packet:
		dataFlowSet, _, _, optionsDataFlowSet = producer.SplitNetFlowSets(msgDecConv)
		obsDomainID = msgDecConv.SourceId
		version = 9
	case netflow.IPFIXPacket:
		dataFlowSet, _, _, optionsDataFlowSet = producer.SplitIPFIXSets(msgDecConv)
		obsDomainID = msgDecConv.ObservationDomainId
		version = 10
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
			flow := decodeRecord(version, record.Values)
			if flow != nil {
				flow.SamplingRate = samplingRate
				flowMessageSet = append(flowMessageSet, flow)
			}
		}
	}

	return flowMessageSet
}

func decodeRecord(version int, fields []netflow.DataField) *schema.FlowMessage {
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
			schema.Flows.ProtobufAppendVarint(bf, schema.ColumnBytes, decodeUNumber(v))
		case netflow.NFV9_FIELD_IN_PKTS, netflow.NFV9_FIELD_OUT_PKTS:
			schema.Flows.ProtobufAppendVarint(bf, schema.ColumnPackets, decodeUNumber(v))

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
			schema.Flows.ProtobufAppendVarint(bf, schema.ColumnSrcNetMask, decodeUNumber(v))
		case netflow.NFV9_FIELD_DST_MASK, netflow.NFV9_FIELD_IPV6_DST_MASK:
			schema.Flows.ProtobufAppendVarint(bf, schema.ColumnDstNetMask, decodeUNumber(v))
		case netflow.NFV9_FIELD_IPV4_NEXT_HOP, netflow.NFV9_FIELD_BGP_IPV4_NEXT_HOP, netflow.NFV9_FIELD_IPV6_NEXT_HOP, netflow.NFV9_FIELD_BGP_IPV6_NEXT_HOP:
			bf.NextHop = decodeIP(v)

		// L4
		case netflow.NFV9_FIELD_L4_SRC_PORT:
			schema.Flows.ProtobufAppendVarint(bf, schema.ColumnSrcPort, decodeUNumber(v))
		case netflow.NFV9_FIELD_L4_DST_PORT:
			schema.Flows.ProtobufAppendVarint(bf, schema.ColumnDstPort, decodeUNumber(v))
		case netflow.NFV9_FIELD_PROTOCOL:
			schema.Flows.ProtobufAppendVarint(bf, schema.ColumnProto, decodeUNumber(v))

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
			schema.Flows.ProtobufAppendVarint(bf, schema.ColumnForwardingStatus, decodeUNumber(v))
		}
	}
	schema.Flows.ProtobufAppendVarint(bf, schema.ColumnEType, uint64(etype))
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
