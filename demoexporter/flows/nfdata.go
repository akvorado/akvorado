// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"bytes"
	"context"
	"encoding/binary"
	"time"

	"akvorado/common/helpers"
)

// getNetFlowData will transform the generated flows into UDP payloads
// to be sent on the wire. It returns the payloads on a channel. All
// messages should be read to avoid leaking the channel.
func getNetFlowData(ctx context.Context, flows []generatedFlow, sequenceNumber uint32, start, now time.Time) <-chan []byte {
	output := make(chan []byte, 16)
	uptime := uint32(now.Sub(start).Seconds())

	// We have to seperate IPv6 and IPv4 flows
	ipFlows := map[uint16][]*generatedFlow{
		helpers.ETypeIPv4: make([]*generatedFlow, 0, len(flows)),
		helpers.ETypeIPv6: make([]*generatedFlow, 0, len(flows)),
	}
	for idx := range flows {
		etype := flows[idx].EType
		ipFlows[etype] = append(ipFlows[etype], &flows[idx])
	}
	go func() {
		for _, etype := range []uint16{helpers.ETypeIPv4, helpers.ETypeIPv6} {
			flows := ipFlows[etype]
			settings := flowSettings[etype]
			for i := 0; i < len(flows); i += settings.MaxFlowsPerPacket {
				upper := i + settings.MaxFlowsPerPacket
				if upper > len(flows) {
					upper = len(flows)
				}
				fls := flows[i:upper]
				buf := new(bytes.Buffer)
				if err := binary.Write(buf, binary.BigEndian, nfv9Header{
					Version:        9,
					Count:          uint16(len(fls)),
					SystemUptime:   uptime,
					UnixSeconds:    uint32(now.Unix()),
					SequenceNumber: sequenceNumber,
					SourceID:       0,
				}); err != nil {
					panic(err)
				}
				if err := binary.Write(buf, binary.BigEndian, flowSetHeader{
					Id:     settings.TemplateID,
					Length: uint16(len(fls)*settings.FlowLength + 4),
				}); err != nil {
					panic(err)
				}
				for _, flow := range fls {
					flow.StartTime = uptime
					flow.EndTime = uptime
					flow.SamplerID = 1
					var err error
					if etype == helpers.ETypeIPv4 {
						err = binary.Write(buf, binary.BigEndian, ipv4Flow{
							IPFlow:  flow.IPFlow,
							SrcAddr: flow.SrcAddr.As4(),
							DstAddr: flow.DstAddr.As4(),
						})
					} else {
						err = binary.Write(buf, binary.BigEndian, ipv6Flow{
							IPFlow:  flow.IPFlow,
							SrcAddr: flow.SrcAddr.As16(),
							DstAddr: flow.DstAddr.As16(),
						})
					}
					if err != nil {
						panic(err)
					}
				}
				select {
				case output <- buf.Bytes():
				case <-ctx.Done():
					return
				}
				sequenceNumber++
			}
		}
		defer close(output)
	}()
	return output
}
