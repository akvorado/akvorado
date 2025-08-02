// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/bits"
	"math/rand"
	"net/netip"
	"time"

	"akvorado/common/helpers"
)

// GeneratedFlow represents a generated flow.
type generatedFlow struct {
	IPFlow
	EType   uint16
	SrcAddr netip.Addr
	DstAddr netip.Addr
}

// rateToCount converts a per-second rate to the number of items to
// produce for the given time.
func rateToCount(rate float64, now time.Time) int {
	seconds := float64(now.Unix() - now.Truncate(time.Hour*24*30*12).Unix())
	count := math.Trunc((seconds+1)*rate) - math.Trunc(seconds*rate)
	return int(count)
}

// randomIP returns a random IP in the provided prefix.
func randomIP(prefix netip.Prefix, r *rand.Rand) netip.Addr {
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
	addr, _ := netip.AddrFromSlice(result)
	return addr
}

// peakHourDistance returns distance from peak hour (0 to 1)
func peakHourDistance(now, peak time.Duration) float64 {
	delta := math.Mod(math.Abs((now - peak).Hours()), 24)
	if 24-delta < delta {
		delta = 24 - delta
	}
	return (12 - delta) / 12
}

// chooseRandom returns a random value from a slice
func chooseRandom[T any](r *rand.Rand, slice []T) T {
	if len(slice) == 0 {
		var result T
		return result
	}
	if len(slice) == 1 {
		return slice[0]
	}
	return slice[r.Intn(len(slice))]
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
		square := distance * distance
		multiplier := 1 + (flowConfig.Multiplier-1)*square/(2.*(square-distance)+1.)
		count := rateToCount(flowConfig.PerSecond*multiplier*(0.9+r.Float64()/5), now)
		for ; count > 0; count-- {
			flow := generatedFlow{
				IPFlow: IPFlow{
					Packets:       1,
					InputInt:      uint32(chooseRandom(r, flowConfig.InIfIndex)),
					OutputInt:     uint32(chooseRandom(r, flowConfig.OutIfIndex)),
					SrcAS:         chooseRandom(r, flowConfig.SrcAS),
					DstAS:         chooseRandom(r, flowConfig.DstAS),
					ForwardStatus: 64,
				},
			}
			if flowConfig.Size == 0 {
				flow.Octets = uint32(r.Int31n(1200) + 300)
			} else {
				flow.Octets = uint32(float64(flowConfig.Size) * (r.NormFloat64()*0.3 + 1))
				if flow.Octets > 9000 {
					flow.Octets = 9000
				} else if flow.Octets > 1500 && flowConfig.Size <= 1500 {
					flow.Octets = 1500
				}
			}
			flow.SrcAddr = randomIP(flowConfig.SrcNet, r)
			flow.SrcMask = uint8(flowConfig.SrcNet.Bits())
			flow.DstAddr = randomIP(flowConfig.DstNet, r)
			flow.DstMask = uint8(flowConfig.DstNet.Bits())
			proto := chooseRandom(r, flowConfig.Protocol)
			if proto == "tcp" || proto == "udp" {
				if srcPort := chooseRandom(r, flowConfig.SrcPort); srcPort != 0 {
					flow.SrcPort = srcPort
				} else {
					flow.SrcPort = uint16(r.Int31n(2000) + 33000)
				}
				if dstPort := chooseRandom(r, flowConfig.DstPort); dstPort != 0 {
					flow.DstPort = dstPort
				} else {
					flow.DstPort = uint16(r.Int31n(2000) + 33000)
				}
			}
			if flow.SrcAddr.Is4() {
				flow.EType = helpers.ETypeIPv4
			} else {
				flow.EType = helpers.ETypeIPv6
			}
			if proto == "tcp" {
				flow.Proto = 6
			} else if proto == "udp" {
				flow.Proto = 17
			} else if proto == "icmp" && flow.EType == helpers.ETypeIPv4 {
				flow.Proto = 1
			} else if proto == "icmp" && flow.EType == helpers.ETypeIPv6 {
				flow.Proto = 58
			}
			flows = append(flows, flow)
			if flowConfig.ReverseDirectionRatio > 0 {
				reverseFlow := flow
				reverseFlow.Octets = uint32(float32(reverseFlow.Octets) * flowConfig.ReverseDirectionRatio)
				reverseFlow.DstAS, reverseFlow.SrcAS = reverseFlow.SrcAS, reverseFlow.DstAS
				reverseFlow.SrcAddr, reverseFlow.DstAddr = reverseFlow.DstAddr, reverseFlow.SrcAddr
				reverseFlow.SrcMask, reverseFlow.DstMask = reverseFlow.DstMask, reverseFlow.SrcMask
				reverseFlow.SrcPort, reverseFlow.DstPort = reverseFlow.DstPort, reverseFlow.SrcPort
				reverseFlow.InputInt, reverseFlow.OutputInt = reverseFlow.OutputInt, reverseFlow.InputInt
				flows = append(flows, reverseFlow)
			}
		}
	}
	return flows
}
