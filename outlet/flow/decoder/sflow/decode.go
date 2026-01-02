// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-FileCopyrightText: 2021 NetSampler
// SPDX-License-Identifier: AGPL-3.0-only AND BSD-3-Clause

package sflow

import (
	"net"

	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"

	"github.com/netsampler/goflow2/v2/decoders/sflow"
)

const (
	// interfaceLocal is used for InIf and OutIf when the traffic is
	// locally originated or terminated. We need to translate it to 0.
	interfaceLocal = 0x3fffffff
	// interfaceFormatIfIndex is used when the interface is an index
	interfaceFormatIfIndex = 0
	// interfaceFormatDiscard is used for OutIf when the traffic is discarded
	interfaceFormatDiscard = 1
	// interfaceFomratMultiple is used when there are multiple output interfaces
	interfaceFormatMultiple = 2
)

func (nd *Decoder) decode(exporter string, packet sflow.Packet, options decoder.Options, bf *schema.FlowMessage, finalize decoder.FinalizeFlowFunc) error {
	for _, flowSample := range packet.Samples {
		var records []sflow.FlowRecord
		forwardingStatus := 0
		switch flowSample := flowSample.(type) {
		case sflow.FlowSample:
			records = flowSample.Records
			bf.SamplingRate = uint64(flowSample.SamplingRate)
			switch flowSample.Input >> 30 {
			case interfaceFormatIfIndex:
				bf.InIf = flowSample.Input
			}
			switch flowSample.Output >> 30 {
			case interfaceFormatIfIndex:
				bf.OutIf = flowSample.Output
			case interfaceFormatDiscard:
				forwardingStatus = 128
			case interfaceFormatMultiple:
			}
		case sflow.ExpandedFlowSample:
			records = flowSample.Records
			bf.SamplingRate = uint64(flowSample.SamplingRate)
			switch flowSample.InputIfFormat {
			case interfaceFormatIfIndex:
				bf.InIf = flowSample.InputIfValue
			}
			switch flowSample.OutputIfFormat {
			case interfaceFormatIfIndex:
				bf.OutIf = flowSample.OutputIfValue
			case interfaceFormatDiscard:
				forwardingStatus = 128
			case interfaceFormatMultiple:
			}
		}

		if bf.InIf == interfaceLocal {
			bf.InIf = 0
		}
		if bf.OutIf == interfaceLocal {
			bf.OutIf = 0
		}

		// Optimization: avoid parsing sampled header if we have everything already parsed
		hasSampledIPv4 := false
		hasSampledIPv6 := false
		hasSampledEthernet := false
		hasExtendedSwitch := false
		needDecap := options.DecapsulationProtocol != pb.RawFlow_DECAP_NONE
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
				//  - we're missing both sampled IPv4 and IPv6 headers, or
				//  - we need L2 data and are missing either sampled ethernet or extended switch, or
				//  - we need L3/L4 data, or
				//  - we have an encapsulation
				needsIPData := !(hasSampledIPv4 || hasSampledIPv6)
				needsL2Data := !(nd.d.Schema.IsDisabled(schema.ColumnGroupL2) || (hasSampledEthernet && hasExtendedSwitch))
				needsL3L4Data := !nd.d.Schema.IsDisabled(schema.ColumnGroupL3L4)
				if needsIPData || needsL2Data || needsL3L4Data || needDecap {
					if l := nd.parseSampledHeader(bf, options.DecapsulationProtocol, &recordData); l > 0 {
						l3length = l
					}
				}
			case sflow.SampledIPv4:
				if needDecap {
					continue
				}
				bf.SrcAddr = decoder.DecodeIP(recordData.SrcIP)
				bf.DstAddr = decoder.DecodeIP(recordData.DstIP)
				l3length = uint64(recordData.Length)
				bf.AppendUint(schema.ColumnProto, uint64(recordData.Protocol))
				bf.AppendUint(schema.ColumnSrcPort, uint64(recordData.SrcPort))
				bf.AppendUint(schema.ColumnDstPort, uint64(recordData.DstPort))
				bf.AppendUint(schema.ColumnEType, helpers.ETypeIPv4)
				bf.AppendUint(schema.ColumnIPTos, uint64(recordData.Tos))
			case sflow.SampledIPv6:
				if needDecap {
					continue
				}
				bf.SrcAddr = decoder.DecodeIP(recordData.SrcIP)
				bf.DstAddr = decoder.DecodeIP(recordData.DstIP)
				l3length = uint64(recordData.Length)
				bf.AppendUint(schema.ColumnProto, uint64(recordData.Protocol))
				bf.AppendUint(schema.ColumnSrcPort, uint64(recordData.SrcPort))
				bf.AppendUint(schema.ColumnDstPort, uint64(recordData.DstPort))
				bf.AppendUint(schema.ColumnEType, helpers.ETypeIPv6)
				bf.AppendUint(schema.ColumnIPTos, uint64(recordData.Priority))
			case sflow.SampledEthernet:
				if needDecap {
					continue
				}
				if l3length == 0 {
					// That's the best we can guess. sFlow says: For a layer 2
					// header_protocol, length is total number of octets of data
					// received on the network (excluding framing bits but
					// including FCS octets).
					l3length = uint64(recordData.Length) - 16
				}
				if !nd.d.Schema.IsDisabled(schema.ColumnGroupL2) {
					bf.AppendUint(schema.ColumnSrcMAC, helpers.MACToUint64(net.HardwareAddr(recordData.SrcMac)))
					bf.AppendUint(schema.ColumnDstMAC, helpers.MACToUint64(net.HardwareAddr(recordData.DstMac)))
				}
			case sflow.ExtendedSwitch:
				if needDecap {
					continue
				}
				if !nd.d.Schema.IsDisabled(schema.ColumnGroupL2) {
					if recordData.SrcVlan < 4096 {
						bf.SrcVlan = uint16(recordData.SrcVlan)
					}
					if recordData.DstVlan < 4096 {
						bf.DstVlan = uint16(recordData.DstVlan)
					}
				}
			case sflow.ExtendedRouter:
				if needDecap {
					continue
				}
				bf.SrcNetMask = uint8(recordData.SrcMaskLen)
				bf.DstNetMask = uint8(recordData.DstMaskLen)
				bf.NextHop = decoder.DecodeIP(recordData.NextHop)
			case sflow.ExtendedGateway:
				if needDecap {
					continue
				}
				bf.NextHop = decoder.DecodeIP(recordData.NextHop)
				bf.DstAS = recordData.AS
				bf.SrcAS = recordData.AS
				if recordData.SrcAS > 0 {
					bf.SrcAS = recordData.SrcAS
				}
				if len(recordData.ASPath) > 0 {
					bf.DstAS = recordData.ASPath[len(recordData.ASPath)-1]
					bf.AppendArrayUInt32(schema.ColumnDstASPath, recordData.ASPath)
				}
				if len(recordData.Communities) > 0 {
					bf.AppendArrayUInt32(schema.ColumnDstCommunities, recordData.Communities)
				}
			}
		}

		if l3length > 0 {
			bf.AppendUint(schema.ColumnBytes, l3length)
		} else if needDecap {
			// This is not 100% true, but this should be good enough.
			nd.metrics.errors.WithLabelValues(exporter, "non-encapsulated packet").Inc()
			continue
		}

		bf.ExporterAddress = decoder.DecodeIP(packet.AgentIP)
		bf.AppendUint(schema.ColumnPackets, 1)
		bf.AppendUint(schema.ColumnForwardingStatus, uint64(forwardingStatus))
		finalize()
	}

	return nil
}

func (nd *Decoder) parseSampledHeader(bf *schema.FlowMessage, decap pb.RawFlow_DecapsulationProtocol, header *sflow.SampledHeader) uint64 {
	data := header.HeaderData
	switch header.Protocol {
	case 1: // Ethernet
		return decoder.ParseEthernet(nd.d.Schema, bf, decap, data)
	case 11: // IPv4
		return decoder.ParseIPv4(nd.d.Schema, bf, decap, data)
	case 12: // IPv6
		return decoder.ParseIPv6(nd.d.Schema, bf, decap, data)
	}
	return 0
}
