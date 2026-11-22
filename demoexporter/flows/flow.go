// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"math"
	"net/netip"
	"time"

	"akvorado/demoexporter/flowmodel"
)

// generatedFlow represents a generated flow.
type generatedFlow struct {
	IPFlow
	EType   uint16
	SrcAddr netip.Addr
	DstAddr netip.Addr
}

// generate produces one second worth of flows for this instance: the
// model samples the flows of the whole fleet and only the ones
// assigned to the configured exporter are kept.
func (c *Component) generate(now time.Time) []generatedFlow {
	// Only the flows assigned to this exporter are fully sampled.
	flows := c.sampler.Generate(c.config.Seed, now, func(flow *flowmodel.Flow) bool {
		exporter, externalIf, ok := c.mapping.assign(flow)
		if !ok || exporter != c.config.Exporter {
			return false
		}
		flow.Tag = externalIf
		return true
	})
	result := make([]generatedFlow, 0, len(flows))
	for i := range flows {
		flow := &flows[i]
		externalIf := flow.Tag
		otherIf := c.mapping.otherInterface(c.config.Exporter, flow.LocalBoundary, externalIf, flow.Key)
		inIf, outIf := externalIf, otherIf
		if flow.Direction == flowmodel.DirectionOutbound {
			inIf, outIf = otherIf, externalIf
		}
		result = append(result, generatedFlow{
			Packets:       flow.Packets,
			Octets:        uint32(min(flow.Octets, math.MaxUint32)),
			InputInt:      uint32(inIf),
			OutputInt:     uint32(outIf),
			SrcPort:       flow.SrcPort,
			DstPort:       flow.DstPort,
			SrcAS:         flow.SrcAS,
			DstAS:         flow.DstAS,
			Proto:         flow.Proto,
			ForwardStatus: 64,
			SrcMask:       flow.SrcMask,
			DstMask:       flow.DstMask,
			EType:         flow.EType,
			SrcAddr:       flow.SrcAddr,
			DstAddr:       flow.DstAddr,
		})
	}
	return result
}
