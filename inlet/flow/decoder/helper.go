// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

import (
	"net"

	goflowmessage "github.com/netsampler/goflow2/pb"
)

// ConvertGoflowToFlowMessage a flow message from goflow2 to our own
// format.
func ConvertGoflowToFlowMessage(input *goflowmessage.FlowMessage) *FlowMessage {
	result := FlowMessage{
		TimeReceived:     input.TimeReceived,
		SequenceNum:      input.SequenceNum,
		SamplingRate:     input.SamplingRate,
		FlowDirection:    input.FlowDirection,
		ExporterAddress:  ipCopy(input.SamplerAddress),
		TimeFlowStart:    input.TimeFlowStart,
		TimeFlowEnd:      input.TimeFlowEnd,
		Bytes:            input.Bytes,
		Packets:          input.Packets,
		SrcAddr:          ipCopy(input.SrcAddr),
		DstAddr:          ipCopy(input.DstAddr),
		Etype:            input.Etype,
		Proto:            input.Proto,
		SrcPort:          input.SrcPort,
		DstPort:          input.DstPort,
		InIf:             input.InIf,
		OutIf:            input.OutIf,
		IPTos:            input.IPTos,
		ForwardingStatus: input.ForwardingStatus,
		IPTTL:            input.IPTTL,
		TCPFlags:         input.TCPFlags,
		IcmpType:         input.IcmpType,
		IcmpCode:         input.IcmpCode,
		IPv6FlowLabel:    input.IPv6FlowLabel,
		FragmentId:       input.FragmentId,
		FragmentOffset:   input.FragmentOffset,
		BiFlowDirection:  input.BiFlowDirection,
		SrcAS:            input.SrcAS,
		DstAS:            input.DstAS,
		SrcNetMask:       input.SrcNet,
		DstNetMask:       input.DstNet,
		NextHopAS:        input.NextHopAS,
		// Note casing of ID in VlanID changes here to make
		// golint happy wherever we use it in future
		VlanID: input.VlanId,
	}
	if !net.IP(input.BgpNextHop).IsUnspecified() {
		result.NextHop = ipCopy(input.BgpNextHop)
	} else {
		result.NextHop = ipCopy(input.NextHop)
	}
	return &result
}

// Ensure we copy the IP address. This is similar to To16(), except
// that when we get an IPv6, we return a copy.
func ipCopy(src net.IP) net.IP {
	if len(src) == 4 {
		return net.IPv4(src[0], src[1], src[2], src[3])
	}
	if len(src) == 16 {
		dst := make(net.IP, len(src))
		copy(dst, src)
		return dst
	}
	return nil
}
