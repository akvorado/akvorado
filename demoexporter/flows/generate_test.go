// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"fmt"
	"math"
	"math/rand"
	"net/netip"
	"testing"
	"time"

	"akvorado/common/helpers"
)

func TestRateToCount(t *testing.T) {
	rates := []float64{0.2, 0.4, 0.6, 1, 1.4, 2.3, 2.8, 3, 3.2, 4.7, 1200}
	now := time.Now().Truncate(time.Second)
	for _, rate := range rates {
		result := 0
		for range 1000 {
			result += rateToCount(rate, now)
			now = now.Add(time.Second)
		}
		computedRate := float64(result) / 1000
		if math.Abs(rate-computedRate) > rate*0.01 {
			t.Errorf("rateToCount(%f) was %f on average", rate, computedRate)
		}
	}
}

func TestRandomIP(t *testing.T) {
	prefixes := []string{
		"192.168.0.0/24",
		"192.168.0.0/16",
		"172.16.0.0/12",
		"192.168.14.1/32",
		"0.0.0.0/0",
		"2001:db8::/32",
		"2001:db8:a:b::/64",
		"2001:db8:a:c:d::1/128",
	}
	r := rand.New(rand.NewSource(0))
	for _, p := range prefixes {
		prefix := netip.MustParsePrefix(p)
		for range 1000 {
			ip := randomIP(prefix, r)
			if !prefix.Contains(ip) {
				t.Errorf("randomIP(%q) == %q not in prefix", p, ip)
				break
			}
		}
	}
}

func TestPeakHourDistance(t *testing.T) {
	cases := []struct {
		Pos      helpers.Pos
		Peak     time.Duration
		Now      time.Duration
		Expected float64
	}{
		{helpers.Mark(), 6 * time.Hour, 6 * time.Hour, 1},
		{helpers.Mark(), 6 * time.Hour, 0, 0.5},
		{helpers.Mark(), 6 * time.Hour, 18 * time.Hour, 0},
		{helpers.Mark(), 12 * time.Hour, 13 * time.Hour, 11. / 12},
		{helpers.Mark(), 12 * time.Hour, 11 * time.Hour, 11. / 12},
		{helpers.Mark(), 12 * time.Hour, 14 * time.Hour, 10. / 12},
		{helpers.Mark(), 12 * time.Hour, 15 * time.Hour, 9. / 12},
		{helpers.Mark(), 12 * time.Hour, 16 * time.Hour, 8. / 12},
		{helpers.Mark(), 12 * time.Hour, 17 * time.Hour, 7. / 12},
		{helpers.Mark(), 12 * time.Hour, 18 * time.Hour, 6. / 12},
		{helpers.Mark(), 12 * time.Hour, 19 * time.Hour, 5. / 12},
	}
	for _, tc := range cases {
		got := peakHourDistance(tc.Now, tc.Peak)
		if math.Abs(got-tc.Expected) > tc.Expected*0.01 {
			t.Errorf("%speakHourDistance(%s, %s) == %f, expected %f",
				tc.Pos, tc.Peak, tc.Now, got, tc.Expected)
		}
	}
}

func TestChooseRandom(t *testing.T) {
	cases := [][]int{
		nil,
		{},
		{6},
		{1, 2, 3, 4, 10, 12},
	}
	r := rand.New(rand.NewSource(0))
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v", tc), func(t *testing.T) {
			results := map[int]bool{}
			for range 100 {
				result := chooseRandom(r, tc)
				results[result] = true
				if len(tc) == 0 {
					if result != 0 {
						t.Fatalf("chooseRandom() == %d instead of 0", result)
					}
					break
				}
				found := false
				for _, v := range tc {
					if v == result {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("chooseRandom() returned %d, not in slice",
						result)
				}
			}
			if len(tc) != 0 && len(results) != len(tc) {
				t.Fatalf("chooseRandom() did not explore all results (only %d)",
					len(results))
			}
		})
	}
}

func TestGenerateFlows(t *testing.T) {
	cases := []struct {
		Pos helpers.Pos
		FlowConfiguration
		Expected []generatedFlow
	}{
		{
			Pos: helpers.Mark(),
			FlowConfiguration: FlowConfiguration{
				PerSecond:  1,
				InIfIndex:  []int{10},
				OutIfIndex: []int{20, 21},
				PeakHour:   21 * time.Hour,
				Multiplier: 3.1, // 6 hours from peak time → ~2
				SrcNet:     netip.MustParsePrefix("192.0.2.0/24"),
				DstNet:     netip.MustParsePrefix("203.0.113.0/24"),
				SrcAS:      []uint32{65201},
				DstAS:      []uint32{65202},
				SrcPort:    []uint16{443},
				Protocol:   []string{"tcp"},
				Size:       1400,
			},
			Expected: []generatedFlow{
				{
					SrcAddr: netip.MustParseAddr("192.0.2.36"),
					DstAddr: netip.MustParseAddr("203.0.113.91"),
					EType:   0x800,
					IPFlow: IPFlow{
						Octets:        1365,
						Packets:       1,
						Proto:         6,
						SrcPort:       443,
						DstPort:       34905,
						InputInt:      10,
						OutputInt:     21,
						SrcAS:         65201,
						DstAS:         65202,
						ForwardStatus: 64,
						SrcMask:       24,
						DstMask:       24,
					},
				}, {
					SrcAddr: netip.MustParseAddr("192.0.2.30"),
					DstAddr: netip.MustParseAddr("203.0.113.220"),
					EType:   0x800,
					IPFlow: IPFlow{
						Octets:        1500,
						Packets:       1,
						Proto:         6,
						SrcPort:       443,
						DstPort:       33618,
						InputInt:      10,
						OutputInt:     21,
						SrcAS:         65201,
						DstAS:         65202,
						ForwardStatus: 64,
						SrcMask:       24,
						DstMask:       24,
					},
				},
			},
		}, {
			Pos: helpers.Mark(),
			FlowConfiguration: FlowConfiguration{
				PerSecond:  1,
				InIfIndex:  []int{20},
				OutIfIndex: []int{10, 11},
				PeakHour:   3 * time.Hour,
				Multiplier: 4, // 12 hours from peak time → ~1
				SrcNet:     netip.MustParsePrefix("2001:db8::1/128"),
				DstNet:     netip.MustParsePrefix("2001:db8:2::/64"),
				SrcAS:      []uint32{65201},
				DstAS:      []uint32{65202},
				DstPort:    []uint16{443},
				Protocol:   []string{"tcp"},
				Size:       1200,
			},
			Expected: []generatedFlow{
				{
					SrcAddr: netip.MustParseAddr("2001:db8::1"),
					DstAddr: netip.MustParseAddr("2001:db8:2:0:245b:11f7:351e:dc1a"),
					EType:   0x86dd,
					IPFlow: IPFlow{
						Octets:        1170,
						Packets:       1,
						Proto:         6,
						SrcPort:       34045,
						DstPort:       443,
						InputInt:      20,
						OutputInt:     11,
						SrcAS:         65201,
						DstAS:         65202,
						ForwardStatus: 64,
						SrcMask:       128,
						DstMask:       64,
					},
				},
			},
		}, {
			Pos: helpers.Mark(),
			FlowConfiguration: FlowConfiguration{
				PerSecond:             1,
				InIfIndex:             []int{20},
				OutIfIndex:            []int{10, 11},
				PeakHour:              3 * time.Hour,
				Multiplier:            4, // 12 hours from peak time → ~1
				SrcNet:                netip.MustParsePrefix("2001:db8::1/128"),
				DstNet:                netip.MustParsePrefix("2001:db8:2::/64"),
				SrcAS:                 []uint32{65201},
				DstAS:                 []uint32{65202},
				DstPort:               []uint16{443},
				Protocol:              []string{"tcp"},
				Size:                  1200,
				ReverseDirectionRatio: 0.1,
			},
			Expected: []generatedFlow{
				{
					SrcAddr: netip.MustParseAddr("2001:db8::1"),
					DstAddr: netip.MustParseAddr("2001:db8:2:0:245b:11f7:351e:dc1a"),
					EType:   0x86dd,
					IPFlow: IPFlow{
						Octets:        1170,
						Packets:       1,
						Proto:         6,
						SrcPort:       34045,
						DstPort:       443,
						InputInt:      20,
						OutputInt:     11,
						SrcAS:         65201,
						DstAS:         65202,
						ForwardStatus: 64,
						SrcMask:       128,
						DstMask:       64,
					},
				}, {
					DstAddr: netip.MustParseAddr("2001:db8::1"),
					SrcAddr: netip.MustParseAddr("2001:db8:2:0:245b:11f7:351e:dc1a"),
					EType:   0x86dd,
					IPFlow: IPFlow{
						Octets:        1170 / 10,
						Packets:       1,
						Proto:         6,
						DstPort:       34045,
						SrcPort:       443,
						OutputInt:     20,
						InputInt:      11,
						DstAS:         65201,
						SrcAS:         65202,
						ForwardStatus: 64,
						DstMask:       128,
						SrcMask:       64,
					},
				},
			},
		},
	}
	now := time.Date(2022, 3, 18, 15, 0, 0, 0, time.UTC)
	for _, tc := range cases {
		t.Run(fmt.Sprintf("case %s", tc.Pos), func(t *testing.T) {
			got := generateFlows([]FlowConfiguration{tc.FlowConfiguration}, 0, now)
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("%sgeneratedFlows() (-got, +want):\n%s", tc.Pos, diff)
			}
		})
	}
}
