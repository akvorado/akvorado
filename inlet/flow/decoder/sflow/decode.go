// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-FileCopyrightText: 2021 NetSampler
// SPDX-License-Identifier: AGPL-3.0-only AND BSD-3-Clause

package sflow

import (
	"encoding/binary"
	"net/netip"

	"akvorado/common/helpers"
	"akvorado/common/schema"

	"github.com/netsampler/goflow2/decoders/sflow"
)

func decode(msgDec interface{}) []*schema.FlowMessage {
	flowMessageSet := []*schema.FlowMessage{}
	switch msgDec.(type) {
	case sflow.Packet:
	default:
		return nil
	}
	packet := msgDec.(sflow.Packet)

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

		bf.ExporterAddress = decodeIP(packet.AgentIP)
		schema.Flows.ProtobufAppendVarint(bf, schema.ColumnPackets, 1)
		schema.Flows.ProtobufAppendVarint(bf, schema.ColumnForwardingStatus, uint64(forwardingStatus))

		for _, record := range records {
			switch recordData := record.Data.(type) {
			case sflow.SampledHeader:
				schema.Flows.ProtobufAppendVarint(bf, schema.ColumnBytes, uint64(recordData.FrameLength))
				parseSampledHeader(bf, &recordData)
			case sflow.SampledIPv4:
				bf.SrcAddr = decodeIP(recordData.Base.SrcIP)
				bf.DstAddr = decodeIP(recordData.Base.DstIP)
				schema.Flows.ProtobufAppendVarint(bf, schema.ColumnBytes, uint64(recordData.Base.Length))
				schema.Flows.ProtobufAppendVarint(bf, schema.ColumnProto, uint64(recordData.Base.Protocol))
				schema.Flows.ProtobufAppendVarint(bf, schema.ColumnSrcPort, uint64(recordData.Base.SrcPort))
				schema.Flows.ProtobufAppendVarint(bf, schema.ColumnDstPort, uint64(recordData.Base.DstPort))
				schema.Flows.ProtobufAppendVarint(bf, schema.ColumnEType, helpers.ETypeIPv4)
			case sflow.SampledIPv6:
				bf.SrcAddr = decodeIP(recordData.Base.SrcIP)
				bf.DstAddr = decodeIP(recordData.Base.DstIP)
				schema.Flows.ProtobufAppendVarint(bf, schema.ColumnBytes, uint64(recordData.Base.Length))
				schema.Flows.ProtobufAppendVarint(bf, schema.ColumnProto, uint64(recordData.Base.Protocol))
				schema.Flows.ProtobufAppendVarint(bf, schema.ColumnSrcPort, uint64(recordData.Base.SrcPort))
				schema.Flows.ProtobufAppendVarint(bf, schema.ColumnDstPort, uint64(recordData.Base.DstPort))
				schema.Flows.ProtobufAppendVarint(bf, schema.ColumnEType, helpers.ETypeIPv6)
			case sflow.ExtendedRouter:
				schema.Flows.ProtobufAppendVarint(bf, schema.ColumnSrcNetMask, uint64(recordData.SrcMaskLen))
				schema.Flows.ProtobufAppendVarint(bf, schema.ColumnDstNetMask, uint64(recordData.DstMaskLen))
				bf.NextHop = decodeIP(recordData.NextHop)
			case sflow.ExtendedGateway:
				bf.NextHop = decodeIP(recordData.NextHop)
				bf.DstAS = recordData.AS
				bf.SrcAS = recordData.AS
				if len(recordData.ASPath) > 0 {
					bf.DstAS = recordData.ASPath[len(recordData.ASPath)-1]
				}
				if recordData.SrcAS > 0 {
					bf.SrcAS = recordData.SrcAS
				}
			}
		}

		flowMessageSet = append(flowMessageSet, bf)
	}

	return flowMessageSet
}

func parseSampledHeader(bf *schema.FlowMessage, header *sflow.SampledHeader) {
	data := header.HeaderData
	switch header.Protocol {
	case 1: // Ethernet
		parseEthernetHeader(bf, data)
	}
}

func parseEthernetHeader(bf *schema.FlowMessage, data []byte) {
	if len(data) < 14 {
		return
	}
	etherType := data[12:14]
	data = data[14:]
	if etherType[0] == 0x81 && etherType[1] == 0x00 {
		// 802.1q
		if len(data) < 4 {
			return
		}
		etherType = data[2:4]
		data = data[4:]
	}
	if etherType[0] == 0x88 && etherType[1] == 0x47 {
		// MPLS
		for {
			if len(data) < 5 {
				return
			}
			label := binary.BigEndian.Uint32(append([]byte{0}, data[:3]...)) >> 4
			bottom := data[2] & 1
			data = data[4:]
			if bottom == 1 || label <= 15 {
				if data[0]&0xf0>>4 == 4 {
					etherType = []byte{0x8, 0x0}
				} else if data[0]&0xf0>>4 == 6 {
					etherType = []byte{0x86, 0xdd}
				} else {
					return
				}
				break
			}
		}
	}
	var proto uint8
	if etherType[0] == 0x8 && etherType[1] == 0x0 {
		// IPv4
		if len(data) < 20 {
			return
		}
		schema.Flows.ProtobufAppendVarint(bf, schema.ColumnEType, helpers.ETypeIPv4)
		bf.SrcAddr = decodeIP(data[12:16])
		bf.DstAddr = decodeIP(data[16:20])
		proto = data[9]
		ihl := int((data[0] & 0xf) * 4)
		if len(data) >= ihl {
			data = data[ihl:]
		} else {
			data = data[:0]
		}
	} else if etherType[0] == 0x86 && etherType[1] == 0xdd {
		// IPv6
		if len(data) < 40 {
			return
		}
		schema.Flows.ProtobufAppendVarint(bf, schema.ColumnEType, helpers.ETypeIPv6)
		bf.SrcAddr = decodeIP(data[8:24])
		bf.DstAddr = decodeIP(data[24:40])
		proto = data[6]
		data = data[40:]
	}
	schema.Flows.ProtobufAppendVarint(bf, schema.ColumnProto, uint64(proto))

	if proto == 6 || proto == 17 {
		if len(data) > 4 {
			schema.Flows.ProtobufAppendVarint(bf, schema.ColumnSrcPort,
				uint64(binary.BigEndian.Uint16(data[0:2])))
			schema.Flows.ProtobufAppendVarint(bf, schema.ColumnDstPort,
				uint64(binary.BigEndian.Uint16(data[2:4])))
		}
	}
}

func decodeIP(b []byte) netip.Addr {
	if ip, ok := netip.AddrFromSlice(b); ok {
		return netip.AddrFrom16(ip.As16())
	}
	return netip.Addr{}
}
