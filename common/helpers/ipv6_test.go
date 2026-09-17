// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers_test

import (
	"net/netip"
	"reflect"
	"runtime"
	"testing"
	"unique"
	"unsafe"

	"golang.org/x/arch/x86/x86asm"

	"akvorado/common/helpers"
	"akvorado/common/helpers/race"
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

// The types below mirror the internals of net/netip. They give the cost of
// AddrTo6 if netip had a Map() method.
type (
	netipUint128 struct{ hi, lo uint64 }

	netipAddrDetail struct {
		isV6   bool
		zoneV6 string
	}

	netipAddr struct {
		addr netipUint128
		z    unique.Handle[netipAddrDetail]
	}
)

var (
	netipZ4    = unique.Make(netipAddrDetail{})
	netipZ6noz = unique.Make(netipAddrDetail{isV6: true})
)

func (ip netipAddr) Is4() bool { return ip.z == netipZ4 }

func (ip netipAddr) Map() netipAddr {
	if ip.Is4() {
		ip.z = netipZ6noz
	}
	return ip
}

// The inputs are package-level and public on purpose. With a local, the
// compiler optimizes part of the work by putting it outside b.Loop().
var (
	BenchIPv4       = netip.MustParseAddr("192.168.1.1")
	BenchNativeIPv4 = netipAddr{netipUint128{0, 0xffff_c0a80101}, netipZ4}
)

func BenchmarkAddrTo6(b *testing.B) {
	_ = BenchNativeIPv4.z.Value().zoneV6 // silence staticcheck

	b.Run("safe", func(b *testing.B) {
		for b.Loop() {
			_ = addrTo6Safe(BenchIPv4)
		}
	})
	b.Run("unsafe", func(b *testing.B) {
		for b.Loop() {
			_ = helpers.AddrTo6(BenchIPv4)
		}
	})
	b.Run("native", func(b *testing.B) {
		for b.Loop() {
			_ = BenchNativeIPv4.Map()
		}
	})
	b.Run("do nothing", func(b *testing.B) {
		for b.Loop() {
		}
	})
}

// instructionCount disassembles the compiled body of fn and counts the
// instructions. The padding between functions is not counted.
func instructionCount(t *testing.T, fn any) int {
	t.Helper()
	pc := reflect.ValueOf(fn).Pointer()

	// Walk forward until we leave fn to get its size
	var size uintptr
	for {
		owner := runtime.FuncForPC(pc + size)
		if owner == nil || owner.Entry() != pc {
			break
		}
		size++
	}

	code := unsafe.Slice((*byte)(unsafe.Pointer(pc)), size)
	count, padding := 0, 0
	for len(code) > 0 {
		inst, err := x86asm.Decode(code, 64)
		if err != nil {
			t.Fatalf("x86asm.Decode() error:\n%+v", err)
		}
		code = code[inst.Len:]
		if inst.Op == x86asm.INT {
			// Functions are padded with INT3. Only count them if more code
			// follows.
			padding++
			continue
		}
		count += padding + 1
		padding = 0
	}
	return count
}

// TestAddrTo6Optimal checks AddrTo6 compiles to as few instructions as a native
// Map() method would, while the safe version needs more.
func TestAddrTo6Optimal(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skipf("no disassembler for %s", runtime.GOARCH)
	}
	if race.Enabled || testing.CoverMode() != "" {
		t.Skip("instrumented build")
	}

	native := instructionCount(t, netipAddr.Map)
	withUnsafe := instructionCount(t, helpers.AddrTo6)
	withoutUnsafe := instructionCount(t, addrTo6Safe)
	t.Logf("instructions: native %d, unsafe %d, safe %d", native, withUnsafe, withoutUnsafe)

	if diff := helpers.Diff(withUnsafe, native); diff != "" {
		t.Errorf("AddrTo6() instruction count (-got, +want):\n%s", diff)
	}
	if withoutUnsafe <= native {
		t.Errorf("addrTo6Safe() instruction count: %d, expected more than %d",
			withoutUnsafe, native)
	}
}
