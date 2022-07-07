// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"fmt"
	"math"
	"math/rand"
	"net"
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
		for i := 0; i < 1000; i++ {
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
		for i := 0; i < 1000; i++ {
			ip := randomIP(prefix, r)
			addr, ok := netip.AddrFromSlice(ip)
			if !ok {
				t.Errorf("randomIP(%q) returned invalid IP", p)
				continue
			}
			if !prefix.Contains(addr) {
				t.Errorf("randomIP(%q) == %q not in prefix", p, addr)
				break
			}
		}
	}
}

func TestPeakHourDistance(t *testing.T) {
	cases := []struct {
		Peak     time.Duration
		Now      time.Duration
		Expected float64
	}{
		{6 * time.Hour, 6 * time.Hour, 1},
		{6 * time.Hour, 0, 0.5},
		{6 * time.Hour, 18 * time.Hour, 0},
		{12 * time.Hour, 13 * time.Hour, 11. / 12},
		{12 * time.Hour, 11 * time.Hour, 11. / 12},
		{12 * time.Hour, 14 * time.Hour, 10. / 12},
		{12 * time.Hour, 15 * time.Hour, 9. / 12},
		{12 * time.Hour, 16 * time.Hour, 8. / 12},
		{12 * time.Hour, 17 * time.Hour, 7. / 12},
		{12 * time.Hour, 18 * time.Hour, 6. / 12},
		{12 * time.Hour, 19 * time.Hour, 5. / 12},
	}
	for _, tc := range cases {
		got := peakHourDistance(tc.Now, tc.Peak)
		if math.Abs(got-tc.Expected) > tc.Expected*0.01 {
			t.Errorf("peakHourDistance(%s, %s) == %f, expected %f",
				tc.Peak, tc.Now, got, tc.Expected)
		}
	}
}

func TestGenerateFlows(t *testing.T) {
	cases := []struct {
		FlowConfiguration
		Expected []generatedFlow
	}{
		{
			FlowConfiguration: FlowConfiguration{
				PerSecond:  1,
				InIfIndex:  10,
				OutIfIndex: 20,
				PeakHour:   21 * time.Hour,
				Multiplier: 3.1, // 6 hours from peak time → ~2
				SrcNet:     netip.MustParsePrefix("192.0.2.0/24"),
				DstNet:     netip.MustParsePrefix("203.0.113.0/24"),
				SrcAS:      65201,
				DstAS:      65202,
				SrcPort:    443,
				DstPort:    0,
				Protocol:   "tcp",
				Size:       1400,
			},
			Expected: []generatedFlow{
				{
					SrcAddr: net.ParseIP("192.0.2.218"),
					DstAddr: net.ParseIP("203.0.113.36"),
					EType:   0x800,
					IPFlow: IPFlow{
						Octets:        1434,
						Packets:       1,
						Proto:         6,
						SrcPort:       443,
						DstPort:       33571,
						InputInt:      10,
						OutputInt:     20,
						SrcAS:         65201,
						DstAS:         65202,
						ForwardStatus: 64,
					},
				}, {
					SrcAddr: net.ParseIP("192.0.2.247"),
					DstAddr: net.ParseIP("203.0.113.53"),
					EType:   0x800,
					IPFlow: IPFlow{
						Octets:        1333,
						Packets:       1,
						Proto:         6,
						SrcPort:       443,
						DstPort:       34758,
						InputInt:      10,
						OutputInt:     20,
						SrcAS:         65201,
						DstAS:         65202,
						ForwardStatus: 64,
					},
				},
			},
		}, {
			FlowConfiguration: FlowConfiguration{
				PerSecond:  1,
				InIfIndex:  20,
				OutIfIndex: 10,
				PeakHour:   3 * time.Hour,
				Multiplier: 4, // 12 hours from peak time → ~1
				SrcNet:     netip.MustParsePrefix("2001:db8::1/128"),
				DstNet:     netip.MustParsePrefix("2001:db8:2::/64"),
				SrcAS:      65201,
				DstAS:      65202,
				SrcPort:    0,
				DstPort:    443,
				Protocol:   "tcp",
				Size:       1200,
			},
			Expected: []generatedFlow{
				{
					SrcAddr: net.ParseIP("2001:db8::1"),
					DstAddr: net.ParseIP("2001:db8:2:0:da24:5b11:f735:1edc"),
					EType:   0x86dd,
					IPFlow: IPFlow{
						Octets:        1229,
						Packets:       1,
						Proto:         6,
						SrcPort:       33618,
						DstPort:       443,
						InputInt:      20,
						OutputInt:     10,
						SrcAS:         65201,
						DstAS:         65202,
						ForwardStatus: 64,
					},
				},
			},
		},
	}
	now := time.Date(2022, 3, 18, 15, 0, 0, 0, time.UTC)
	for i, tc := range cases {
		t.Run(fmt.Sprintf("case %d", i+1), func(t *testing.T) {
			got := generateFlows([]FlowConfiguration{tc.FlowConfiguration}, 0, now)
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("generatedFlows() (-got, +want):\n%s", diff)
			}
		})
	}
}
