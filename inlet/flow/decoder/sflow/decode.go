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

func (nd *Decoder) decode(msgDec interface{}) []*schema.FlowMessage {
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
				bf.SrcAddr = decodeIP(recordData.Base.SrcIP)
				bf.DstAddr = decodeIP(recordData.Base.DstIP)
				l3length = uint64(recordData.Base.Length)
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnProto, uint64(recordData.Base.Protocol))
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnSrcPort, uint64(recordData.Base.SrcPort))
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnDstPort, uint64(recordData.Base.DstPort))
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnEType, helpers.ETypeIPv4)
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPTos, uint64(recordData.Tos))
			case sflow.SampledIPv6:
				bf.SrcAddr = decodeIP(recordData.Base.SrcIP)
				bf.DstAddr = decodeIP(recordData.Base.DstIP)
				l3length = uint64(recordData.Base.Length)
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnProto, uint64(recordData.Base.Protocol))
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnSrcPort, uint64(recordData.Base.SrcPort))
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnDstPort, uint64(recordData.Base.DstPort))
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnEType, helpers.ETypeIPv6)
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPTos, uint64(recordData.Priority))
			case sflow.SampledEthernet:
				if l3length == 0 {
					// That's the best we can guess.
					l3length = uint64(recordData.Length) - 16 // (MACs, ethertype, FCS)
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
				bf.NextHop = decodeIP(recordData.NextHop)
			case sflow.ExtendedGateway:
				bf.NextHop = decodeIP(recordData.NextHop)
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
				}
				bf.GotASPath = true
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
		return nd.parseEthernetHeader(bf, data)
	case 11: // IPv4
		return nd.parseIPv4Header(bf, data)
	case 12: // IPv6
		return nd.parseIPv6Header(bf, data)
	}
	return 0
}

func (nd *Decoder) parseIPv4Header(bf *schema.FlowMessage, data []byte) uint64 {
	var l3length uint64
	var proto uint8
	if len(data) < 20 {
		return 0
	}
	nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnEType, helpers.ETypeIPv4)
	l3length = uint64(binary.BigEndian.Uint16(data[2:4]))
	bf.SrcAddr = decodeIP(data[12:16])
	bf.DstAddr = decodeIP(data[16:20])
	proto = data[9]
	if !nd.d.Schema.IsDisabled(schema.ColumnGroupL3L4) {
		nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPTos, uint64(data[1]))
		nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPTTL, uint64(data[8]))
		nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPFragmentID,
			uint64(binary.BigEndian.Uint16(data[4:6])))
		nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPFragmentOffset,
			uint64(binary.BigEndian.Uint16(data[6:8])&0x1fff))
	}
	ihl := int((data[0] & 0xf) * 4)
	if len(data) >= ihl {
		data = data[ihl:]
	} else {
		data = data[:0]
	}
	nd.parseL4Header(bf, data, proto)
	return l3length
}

func (nd *Decoder) parseIPv6Header(bf *schema.FlowMessage, data []byte) uint64 {
	var l3length uint64
	var proto uint8
	if len(data) < 40 {
		return 0
	}
	l3length = uint64(binary.BigEndian.Uint16(data[4:6])) + 40
	nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnEType, helpers.ETypeIPv6)
	bf.SrcAddr = decodeIP(data[8:24])
	bf.DstAddr = decodeIP(data[24:40])
	proto = data[6]
	nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnProto, uint64(proto))
	if !nd.d.Schema.IsDisabled(schema.ColumnGroupL3L4) {
		nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPTos,
			uint64(binary.BigEndian.Uint16(data[0:2])&0xff0>>4))
		nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPTTL, uint64(data[7]))
		nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnIPv6FlowLabel,
			uint64(binary.BigEndian.Uint32(data[0:4])&0xfffff))
		// TODO fragmentID/fragmentOffset are in a separate header
	}
	data = data[40:]
	nd.parseL4Header(bf, data, proto)
	return l3length
}

func (nd *Decoder) parseL4Header(bf *schema.FlowMessage, data []byte, proto uint8) {
	nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnProto, uint64(proto))
	if proto == 6 || proto == 17 {
		// UDP or TCP
		if len(data) > 4 {
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnSrcPort,
				uint64(binary.BigEndian.Uint16(data[0:2])))
			nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnDstPort,
				uint64(binary.BigEndian.Uint16(data[2:4])))
		}
	}
	if !nd.d.Schema.IsDisabled(schema.ColumnGroupL3L4) {
		if proto == 6 {
			// TCP
			if len(data) > 13 {
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnTCPFlags,
					uint64(data[13]))
			}
		} else if proto == 1 || proto == 58 {
			// ICMP and ICMPv6
			if len(data) > 2 {
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnICMPType,
					uint64(data[0]))
				nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnICMPCode,
					uint64(data[1]))
			}
		}
	}
}

func (nd *Decoder) parseEthernetHeader(bf *schema.FlowMessage, data []byte) uint64 {
	if len(data) < 14 {
		return 0
	}
	if !nd.d.Schema.IsDisabled(schema.ColumnGroupL2) {
		nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnDstMAC,
			binary.BigEndian.Uint64([]byte{0, 0, data[0], data[1], data[2], data[3], data[4], data[5]}))
		nd.d.Schema.ProtobufAppendVarint(bf, schema.ColumnSrcMAC,
			binary.BigEndian.Uint64([]byte{0, 0, data[6], data[7], data[8], data[9], data[10], data[11]}))
	}
	etherType := data[12:14]
	data = data[14:]
	if etherType[0] == 0x81 && etherType[1] == 0x00 {
		// 802.1q
		if len(data) < 4 {
			return 0
		}
		if !nd.d.Schema.IsDisabled(schema.ColumnGroupL2) {
			bf.SrcVlan = (uint16(data[0]&0xf) << 8) + uint16(data[1])
		}
		etherType = data[2:4]
		data = data[4:]
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
		return nd.parseIPv4Header(bf, data)
	} else if etherType[0] == 0x86 && etherType[1] == 0xdd {
		return nd.parseIPv6Header(bf, data)
	}
	return 0
}

func decodeIP(b []byte) netip.Addr {
	if ip, ok := netip.AddrFromSlice(b); ok {
		return netip.AddrFrom16(ip.As16())
	}
	return netip.Addr{}
}
