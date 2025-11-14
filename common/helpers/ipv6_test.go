// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers_test

import (
	"fmt"
	"net/netip"
	"reflect"
	"testing"
	"unsafe"

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

func TestNetIPAddrStructure(t *testing.T) {
	var addr netip.Addr
	addrType := reflect.TypeFor[netip.Addr]()

	// Test total size: 24 bytes (16 for uint128 + 8 for unique.Handle)
	if unsafe.Sizeof(addr) != 24 {
		t.Errorf("netip.Addr size = %d, want 24", unsafe.Sizeof(addr))
	}

	// Test number of fields
	if addrType.NumField() != 2 {
		t.Errorf("netip.Addr has %d fields, want 2", addrType.NumField())
	}

	// Test field 0: addr (uint128, 16 bytes)
	field0 := addrType.Field(0)
	if field0.Name != "addr" {
		t.Errorf("field 0 name = %q, want %q", field0.Name, "addr")
	}
	if field0.Type.String() != "netip.uint128" {
		t.Errorf("field 0 type = %q, want %q", field0.Type.String(), "netip.uint128")
	}
	if field0.Offset != 0 {
		t.Errorf("field 0 offset = %d, want 0", field0.Offset)
	}
	if field0.Type.Size() != 16 {
		t.Errorf("field 0 (addr) size = %d, want 16", field0.Type.Size())
	}

	// Test field 1: z (unique.Handle, 8 bytes)
	field1 := addrType.Field(1)
	if field1.Name != "z" {
		t.Errorf("field 1 name = %q, want %q", field1.Name, "z")
	}
	if field1.Type.String() != "unique.Handle[net/netip.addrDetail]" {
		t.Errorf("field 0 type = %q, want %q", field1.Type.String(), "unique.Handle[net/netip.addrDetail]")
	}
	if field1.Offset != 16 {
		t.Errorf("field 1 offset = %d, want 16", field1.Offset)
	}
	if field1.Type.Size() != 8 {
		t.Errorf("field 1 (z) size = %d, want 8", field1.Type.Size())
	}

	t.Logf("netip.Addr structure verified: [addr %d bytes @ 0] [z %d bytes @ 16]",
		field0.Type.Size(), field1.Type.Size())
}

func addrTo6Safe(ip netip.Addr) netip.Addr {
	if ip.Is4() {
		return netip.AddrFrom16(ip.As16())
	}
	return ip
}

func addrTo6SafeNocheck(ip netip.Addr) netip.Addr {
	return netip.AddrFrom16(ip.As16())
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
		b.Run(fmt.Sprintf("safe nocheck %s", version), func(b *testing.B) {
			for b.Loop() {
				_ = addrTo6SafeNocheck(ip)
			}
		})
		b.Run(fmt.Sprintf("unsafe %s", version), func(b *testing.B) {
			for b.Loop() {
				_ = helpers.AddrTo6(ip)
			}
		})
	}
}
