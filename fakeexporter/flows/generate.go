// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/bits"
	"math/rand"
	"net"
	"net/netip"
	"time"

	"akvorado/common/helpers"
)

// GeneratedFlow represents a generated flow.
type generatedFlow struct {
	IPFlow
	EType   uint16
	SrcAddr net.IP
	DstAddr net.IP
}

// rateToCount convert a per-second rate to the number of items to
// produce for the given time.
func rateToCount(rate float64, now time.Time) int {
	seconds := float64(now.Unix() - now.Truncate(time.Hour*24*30*12).Unix())
	count := math.Trunc((seconds+1)*rate) - math.Trunc(seconds*rate)
	return int(count)
}

// Return a random IP in the provided prefix.
func randomIP(prefix netip.Prefix, r *rand.Rand) net.IP {
	result := make([]byte, prefix.Addr().BitLen()/8)
	for i := range result {
		if prefix.Bits() >= (i+1)*8 {
			result[i] = prefix.Addr().AsSlice()[i]
			continue
		}
		shiftMask := prefix.Bits() - i*8
		if shiftMask < 0 {
			shiftMask = 0
		}
		randomByte := byte(int(r.Int31n(256)))
		randomByte = randomByte & ^bits.Reverse8(byte((1<<shiftMask)-1))
		result[i] = randomByte | prefix.Addr().AsSlice()[i]
	}
	return net.IP(result)
}

// Return distance from peak hour (0 to 1)
func peakHourDistance(now, peak time.Duration) float64 {
	delta := math.Mod(math.Abs((now - peak).Hours()), 24)
	if 24-delta < delta {
		delta = 24 - delta
	}
	return (12 - delta) / 12
}

// generateFlows generate a set of flows using the provided
// configuration, for the provided date. It returns one second worth
// of flows. This is stateless and not very efficient if we have many
// flow configurations.
func generateFlows(flowConfigs []FlowConfiguration, seed int64, now time.Time) []generatedFlow {
	flows := []generatedFlow{}
	now = now.Truncate(time.Second)

	// Initialize the random number generator to a known state
	hash := fnv.New64()
	fmt.Fprintf(hash, "%d %d", now.Unix(), seed)
	r := rand.New(rand.NewSource(int64(hash.Sum64())))

	nowTime := now.Sub(now.Truncate(time.Hour * 24))
	for _, flowConfig := range flowConfigs {
		// Compute how many per seconds
		distance := peakHourDistance(nowTime, flowConfig.PeakHour)
		multiplier := 1 + (flowConfig.Multiplier-1)*distance
		count := rateToCount(flowConfig.PerSecond*multiplier*(0.9+r.Float64()/5), now)
		for ; count > 0; count-- {
			flow := generatedFlow{
				IPFlow: IPFlow{
					Packets:       1,
					InputInt:      uint32(flowConfig.InIfIndex),
					OutputInt:     uint32(flowConfig.OutIfIndex),
					SrcAS:         flowConfig.SrcAS,
					DstAS:         flowConfig.DstAS,
					ForwardStatus: 64,
				},
			}
			if flowConfig.Size == 0 {
				flow.Octets = uint32(r.Int31n(1200) + 300)
			} else {
				flow.Octets = uint32(float64(flowConfig.Size) * (0.9 + r.Float64()/5))
				if flow.Octets > 9000 {
					flow.Octets = 9000
				} else if flow.Octets > 1500 && flowConfig.Size <= 1500 {
					flow.Octets = 1500
				}
			}
			flow.SrcAddr = randomIP(flowConfig.SrcNet, r)
			flow.DstAddr = randomIP(flowConfig.DstNet, r)
			if flowConfig.Protocol == "tcp" || flowConfig.Protocol == "udp" {
				if flowConfig.SrcPort != 0 {
					flow.SrcPort = flowConfig.SrcPort
				} else {
					flow.SrcPort = uint16(r.Int31n(2000) + 33000)
				}
				if flowConfig.DstPort != 0 {
					flow.DstPort = flowConfig.DstPort
				} else {
					flow.DstPort = uint16(r.Int31n(2000) + 33000)
				}
			}
			if flow.SrcAddr.To4() != nil {
				flow.EType = helpers.ETypeIPv4
			} else {
				flow.EType = helpers.ETypeIPv6
			}
			if flowConfig.Protocol == "tcp" {
				flow.Proto = 6
			} else if flowConfig.Protocol == "udp" {
				flow.Proto = 17
			} else if flowConfig.Protocol == "icmp" && flow.EType == helpers.ETypeIPv4 {
				flow.Proto = 1
			} else if flowConfig.Protocol == "icmp" && flow.EType == helpers.ETypeIPv6 {
				flow.Proto = 58
			}
			flows = append(flows, flow)
		}
	}
	return flows
}
