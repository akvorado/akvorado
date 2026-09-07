// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers_test

import (
	"fmt"
	"net/netip"
	"testing"

	"akvorado/common/helpers"
)

func TestAddrTo6(t *testing.T) {
	cases := []struct {
		input  netip.Addr
		output netip.Addr
	}{
		{netip.Addr{}, netip.Addr{}},
		{netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("::ffff:192.168.1.1")},
		{netip.MustParseAddr("2a01:db8::1"), netip.MustParseAddr("2a01:db8::1")},
	}
	for _, tc := range cases {
		got := helpers.AddrTo6(tc.input)
		if diff := helpers.Diff(got, tc.output); diff != "" {
			t.Errorf("AddrTo6(%s) (-got, +want):\n%s", tc.input, diff)
		}
	}
}

func TestPrefixTo6(t *testing.T) {
	cases := []struct {
		input  netip.Prefix
		output netip.Prefix
	}{
		{netip.Prefix{}, netip.Prefix{}},
		{netip.MustParsePrefix("192.168.1.0/24"), netip.MustParsePrefix("::ffff:192.168.1.0/120")},
		{netip.MustParsePrefix("2a01:db8::/64"), netip.MustParsePrefix("2a01:db8::/64")},
	}
	for _, tc := range cases {
		got := helpers.PrefixTo6(tc.input)
		if diff := helpers.Diff(got, tc.output); diff != "" {
			t.Errorf("PrefixTo6(%s) (-got, +want):\n%s", tc.input, diff)
		}
	}
}

func TestUnmapPrefix(t *testing.T) {
	for _, tc := range []struct {
		input  string
		output string
	}{
		{"0.0.0.0/0", "0.0.0.0/0"},
		{"::/0", "::/0"},
		{"192.168.12.0/24", "192.168.12.0/24"},
		{"2001:db8::/52", "2001:db8::/52"},
		{"::ffff:192.168.12.0/120", "192.168.12.0/24"},
		{"::ffff:0.0.0.0/0", "::ffff:0.0.0.0/0"},
		{"::ffff:0.0.0.0/96", "0.0.0.0/0"},
	} {
		prefix := netip.MustParsePrefix(tc.input)
		got := helpers.UnmapPrefix(prefix).String()
		if diff := helpers.Diff(got, tc.output); diff != "" {
			t.Errorf("UnmapPrefix(%q) (-got, +want):\n%s", tc.input, diff)
		}
	}
}

func addrTo6Safe(ip netip.Addr) netip.Addr {
	if ip.Is4() {
		return netip.AddrFrom16(ip.As16())
	}
	return ip
}

func BenchmarkAddrTo6(b *testing.B) {
	ipv4 := netip.MustParseAddr("192.168.1.1")
	ipv6 := netip.MustParseAddr("2a01:db8::1")
	for _, ip := range []netip.Addr{ipv4, ipv6} {
		version := "v4"
		if ip.Is6() {
			version = "v6"
		}
		b.Run(fmt.Sprintf("safe %s", version), func(b *testing.B) {
			for b.Loop() {
				_ = addrTo6Safe(ip)
			}
		})
		b.Run(fmt.Sprintf("unsafe %s", version), func(b *testing.B) {
			for b.Loop() {
				_ = helpers.AddrTo6(ip)
			}
		})
	}
	b.Run("do nothing", func(b *testing.B) {
		for b.Loop() {
		}
	})
}
