// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-FileCopyrightText: 2021 NetSampler
// SPDX-License-Identifier: AGPL-3.0-only AND BSD-3-Clause

package sflow

import (
	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/inlet/flow/decoder"

	"github.com/netsampler/goflow2/v2/decoders/sflow"
)

func (nd *Decoder) decode(packet sflow.Packet) []*schema.FlowMessage {
	flowMessageSet := []*schema.FlowMessage{}

	for _, flowSample := range packet.Samples {
		var records []sflow.FlowRecord
		bf := &schema.FlowMessage{}
		forwardingStatus := 0
		switch flowSample := flowSample.(type) {
		case sflow.FlowSample:
			records = flowSample.Records
			bf.SamplingRate = flowSample.SamplingRate
			bf.InIf = flowSample.Input
			bf.OutIf = flowSample.Output
			if bf.OutIf&interfaceOutMask == interfaceOutDiscard {
				bf.OutIf = 0
				forwardingStatus = 128
			} else if bf.OutIf&interfaceOutMask == interfaceOutMultiple {
				bf.OutIf = 0
			}
		case sflow.ExpandedFlowSample:
			records = flowSample.Records
			bf.SamplingRate = flowSample.SamplingRate
			bf.InIf = flowSample.InputIfValue
			bf.OutIf = flowSample.OutputIfValue
		}

		if bf.InIf == interfaceLocal {
			bf.InIf = 0
		}
		if bf.OutIf == interfaceLocal {
			bf.OutIf = 0
		}

		bf.ExporterAddress = decoder.DecodeIP(packet.AgentIP)
		nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnPackets, 1)
		nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnForwardingStatus, uint64(forwardingStatus))

		// Optimization: avoid parsing sampled header if we have everything already parsed
		hasSampledIPv4 := false
		hasSampledIPv6 := false
		hasSampledEthernet := false
		hasExtendedSwitch := false
		for _, record := range records {
			switch record.Data.(type) {
			case sflow.SampledIPv4:
				hasSampledIPv4 = true
			case sflow.SampledIPv6:
				hasSampledIPv6 = true
			case sflow.SampledEthernet:
				hasSampledEthernet = true
			case sflow.ExtendedSwitch:
				hasExtendedSwitch = true
			}
		}

		var l3length uint64
		for _, record := range records {
			switch recordData := record.Data.(type) {
			case sflow.SampledHeader:
				// Only process this header if:
				//  - we don't have a sampled IPv4 header nor a sampled IPv4 header, or
				//  - we need L2 data and we don't have sampled ethernet header or we don't have extended switch record
				//  - we need L3/L4 data
				if !hasSampledIPv4 && !hasSampledIPv6 || !nd.d.Schema.IsDisabled(schema.ColumnGroupL2) && (!hasSampledEthernet || !hasExtendedSwitch) || !nd.d.Schema.IsDisabled(schema.ColumnGroupL3L4) {
					if l := nd.parseSampledHeader(bf, &recordData); l > 0 {
						l3length = l
					}
				}
			case sflow.SampledIPv4:
				bf.SrcAddr = decoder.DecodeIP(recordData.SrcIP)
				bf.DstAddr = decoder.DecodeIP(recordData.DstIP)
				l3length = uint64(recordData.Length)
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnProto, uint64(recordData.Protocol))
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnSrcPort, uint64(recordData.SrcPort))
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnDstPort, uint64(recordData.DstPort))
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnEType, helpers.ETypeIPv4)
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPTos, uint64(recordData.Tos))
			case sflow.SampledIPv6:
				bf.SrcAddr = decoder.DecodeIP(recordData.SrcIP)
				bf.DstAddr = decoder.DecodeIP(recordData.DstIP)
				l3length = uint64(recordData.Length)
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnProto, uint64(recordData.Protocol))
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnSrcPort, uint64(recordData.SrcPort))
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnDstPort, uint64(recordData.DstPort))
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnEType, helpers.ETypeIPv6)
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPTos, uint64(recordData.Priority))
			case sflow.SampledEthernet:
				if l3length == 0 {
					// That's the best we can guess. sFlow says: For a layer 2
					// header_protocol, length is total number of octets of data
					// received on the network (excluding framing bits but
					// including FCS octets).
					l3length = uint64(recordData.Length) - 16
				}
				if !nd.d.Schema.IsDisabled(schema.ColumnGroupL2) {
					nd.d.Schema.ProtobufAppendBytes(bf, schema.ColumnSrcMAC, recordData.SrcMac)
					nd.d.Schema.ProtobufAppendBytes(bf, schema.ColumnDstMAC, recordData.DstMac)
				}
			case sflow.ExtendedSwitch:
				if !nd.d.Schema.IsDisabled(schema.ColumnGroupL2) {
					if recordData.SrcVlan < 4096 {
						bf.SrcVlan = uint16(recordData.SrcVlan)
					}
					if recordData.DstVlan < 4096 {
						bf.DstVlan = uint16(recordData.DstVlan)
					}
				}
			case sflow.ExtendedRouter:
				bf.SrcNetMask = uint8(recordData.SrcMaskLen)
				bf.DstNetMask = uint8(recordData.DstMaskLen)
				bf.NextHop = decoder.DecodeIP(recordData.NextHop)
			case sflow.ExtendedGateway:
				bf.NextHop = decoder.DecodeIP(recordData.NextHop)
				bf.DstAS = recordData.AS
				bf.SrcAS = recordData.AS
				if recordData.SrcAS > 0 {
					bf.SrcAS = recordData.SrcAS
				}
				if len(recordData.ASPath) > 0 {
					bf.DstAS = recordData.ASPath[len(recordData.ASPath)-1]
					if column, _ := nd.d.Schema.LookupColumnByKey(schema.ColumnDstASPath); !column.Disabled {
						for _, asn := range recordData.ASPath {
							column.ProtobufAppendVarint(bf, uint64(asn))
						}
					}
					bf.GotASPath = true
				}
			}
		}

		if l3length > 0 {
			nd.d.Schema.ProtobufAppendVarintForce(bf, schema.ColumnBytes, l3length)
		}
		flowMessageSet = append(flowMessageSet, bf)
	}

	return flowMessageSet
}

func (nd *Decoder) parseSampledHeader(bf *schema.FlowMessage, header *sflow.SampledHeader) uint64 {
	data := header.HeaderData
	switch header.Protocol {
	case 1: // Ethernet
		return decoder.ParseEthernet(nd.d.Schema, bf, data)
	case 11: // IPv4
		return decoder.ParseIPv4(nd.d.Schema, bf, data)
	case 12: // IPv6
		return decoder.ParseIPv6(nd.d.Schema, bf, data)
	}
	return 0
}
