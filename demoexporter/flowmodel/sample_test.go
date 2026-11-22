// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flowmodel

import (
	"net/netip"
	"testing"
	"time"

	"akvorado/common/constants"
	"akvorado/common/helpers"
)

func TestGenerateDeterminism(t *testing.T) {
	model := NewMock(t)
	sampler1, err := model.NewSampler(SamplerOptions{LocalASN: 64496, Rate: 1, MaxFlowsPerSecond: 5000})
	if err != nil {
		t.Fatalf("NewSampler() error:\n%+v", err)
	}
	sampler2, err := model.NewSampler(SamplerOptions{LocalASN: 64496, Rate: 1, MaxFlowsPerSecond: 5000})
	if err != nil {
		t.Fatalf("NewSampler() error:\n%+v", err)
	}
	now := time.Date(2022, 3, 18, 15, 0, 0, 0, time.UTC)
	for i := range 20 {
		when := now.Add(time.Duration(i) * time.Second)
		first := sampler1.Generate(10, when, nil)
		second := sampler2.Generate(10, when, nil)
		if diff := helpers.Diff(first, second); diff != "" {
			t.Fatalf("Generate() at %s (-got, +want):\n%s", when, diff)
		}
	}
	// A different seed should give different flows at some point.
	same := true
	for i := range 20 {
		when := now.Add(time.Duration(i) * time.Second)
		first := sampler1.Generate(10, when, nil)
		second := sampler2.Generate(11, when, nil)
		if diff := helpers.Diff(first, second); diff != "" {
			same = false
			break
		}
	}
	if same {
		t.Fatalf("Generate() with different seeds always gives the same flows")
	}
}

func TestGenerateValidity(t *testing.T) {
	model := NewMock(t)
	sampler, err := model.NewSampler(SamplerOptions{LocalASN: 64496, Rate: 20, MaxFlowsPerSecond: 5000})
	if err != nil {
		t.Fatalf("NewSampler() error:\n%+v", err)
	}
	now := time.Date(2022, 3, 18, 15, 0, 0, 0, time.UTC)
	total := 0
	for i := range 20 {
		flows := sampler.Generate(0, now.Add(time.Duration(i)*time.Second), nil)
		total += len(flows)
		for _, flow := range flows {
			is4 := flow.EType == constants.ETypeIPv4
			if flow.SrcAddr.Is4() != is4 || flow.DstAddr.Is4() != is4 {
				t.Fatalf("Generate() flow with EType %x and addresses %s → %s",
					flow.EType, flow.SrcAddr, flow.DstAddr)
			}
			if flow.Packets == 0 || flow.Octets == 0 {
				t.Fatalf("Generate() flow with %d packets and %d octets",
					flow.Packets, flow.Octets)
			}
			if flow.Octets < uint64(flow.Packets) {
				t.Fatalf("Generate() flow with more packets (%d) than octets (%d)",
					flow.Packets, flow.Octets)
			}
			local := flow.DstAddr
			if flow.Direction == DirectionOutbound {
				local = flow.SrcAddr
			}
			found := false
			for _, info := range model.Metadata.LocalPrefixes {
				if info.Prefix.Contains(local) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("Generate() flow with local address %s outside the model prefixes", local)
			}
		}
	}
	if total == 0 {
		t.Fatalf("Generate() did not produce any flow")
	}
}

func TestGenerateRate(t *testing.T) {
	model := NewMock(t)
	sampler, err := model.NewSampler(SamplerOptions{Rate: 10, MaxFlowsPerSecond: 5000})
	if err != nil {
		t.Fatalf("NewSampler() error:\n%+v", err)
	}
	now := time.Date(2022, 3, 18, 15, 0, 0, 0, time.UTC)
	total := 0
	for i := range 100 {
		total += len(sampler.Generate(0, now.Add(time.Duration(i)*time.Second), nil))
	}
	// 10 flows per second with a ±10% jitter over 100 seconds.
	if total < 850 || total > 1150 {
		t.Fatalf("Generate() produced %d flows over 100 seconds, expected around 1000", total)
	}
}

func TestHourMix(t *testing.T) {
	cases := []struct {
		Time   time.Time
		How    int
		Other  int
		Weight float64
	}{
		// Friday, 15:00 to 16:00 UTC.
		{time.Date(2022, 3, 18, 15, 30, 0, 0, time.UTC), 135, 136, 0},
		{time.Date(2022, 3, 18, 15, 45, 0, 0, time.UTC), 135, 136, 0.25},
		{time.Date(2022, 3, 18, 15, 15, 0, 0, time.UTC), 135, 134, 0.25},
		{time.Date(2022, 3, 18, 16, 0, 0, 0, time.UTC), 136, 135, 0.5},
		// 17:15 CET is 16:15 UTC.
		{time.Date(2022, 3, 18, 17, 15, 0, 0, time.FixedZone("CET", 3600)), 136, 135, 0.25},
		// Around the end of the week.
		{time.Date(2022, 3, 20, 0, 0, 0, 0, time.UTC), 0, 167, 0.5},
		{time.Date(2022, 3, 19, 23, 45, 0, 0, time.UTC), 167, 0, 0.25},
	}
	for _, tc := range cases {
		how, other, weight := hourMix(tc.Time)
		if diff := helpers.Diff([]any{how, other, weight}, []any{tc.How, tc.Other, tc.Weight}); diff != "" {
			t.Errorf("hourMix(%s) (-got, +want):\n%s", tc.Time, diff)
		}
	}
}

func TestGenerateCap(t *testing.T) {
	model := NewMock(t)
	sampler, err := model.NewSampler(SamplerOptions{Rate: 1000, MaxFlowsPerSecond: 20})
	if err != nil {
		t.Fatalf("NewSampler() error:\n%+v", err)
	}
	now := time.Date(2022, 3, 18, 15, 0, 0, 0, time.UTC)
	for i := range 5 {
		flows := sampler.Generate(0, now.Add(time.Duration(i)*time.Second), nil)
		if len(flows) > 20 {
			t.Fatalf("Generate() produced %d flows, expected at most 20", len(flows))
		}
	}
}

func TestHostIP(t *testing.T) {
	cases := []struct {
		Prefix   string
		Hi, Lo   uint64
		Expected string
	}{
		{"192.0.2.0/24", 0, 0, "192.0.2.0"},
		{"192.0.2.0/24", 0xffffffffffffffff, 0, "192.0.2.255"},
		{"192.0.2.77/24", 0x12345678_00000000, 0, "192.0.2.120"},
		{"2001:db8::/32", 0xffffffffffffffff, 1, "2001:db8:ffff:ffff::1"},
		{"2001:db8::1/128", 0xffffffffffffffff, 0xffffffffffffffff, "2001:db8::1"},
	}
	for _, tc := range cases {
		got := hostIP(netip.MustParsePrefix(tc.Prefix), tc.Hi, tc.Lo)
		if diff := helpers.Diff(got.String(), tc.Expected); diff != "" {
			t.Errorf("hostIP(%s) (-got, +want):\n%s", tc.Prefix, diff)
		}
	}
}

func TestDefaultModel(t *testing.T) {
	model, err := LoadFromPath("")
	if err != nil {
		t.Fatalf("LoadFromPath() error:\n%+v", err)
	}
	sampler, err := model.NewSampler(SamplerOptions{LocalASN: 64496, Rate: 1, MaxFlowsPerSecond: 5000})
	if err != nil {
		t.Fatalf("NewSampler() error:\n%+v", err)
	}
	now := time.Date(2022, 3, 18, 15, 0, 0, 0, time.UTC)
	if flows := sampler.Generate(0, now, nil); len(flows) == 0 {
		t.Fatalf("Generate() did not produce any flow")
	}
}
