// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"net/netip"
	"unsafe"
)

var (
	someIPv6 = netip.MustParseAddr("2001:db8::1")
	z6noz    = *(*uint64)(unsafe.Add(unsafe.Pointer(&someIPv6), 16))
)

// AddrTo6 maps an IPv4 address to an IPv4-mapped IPv6 address. It returns an
// IPv6 address unmodified. This is unsafe, but there is a test to ensure
// netip.Addr is like we expect. Copying a unique.Handle bypass reference count,
// but z6noz is "static".
//
// This would be trivial to implement inside netip:
//
//	func (ip Addr) Unmap() Addr {
//		if ip.Is4() {
//			ip.z = z6noz
//		}
//		return ip
//	}
func AddrTo6(ip netip.Addr) netip.Addr {
	if ip.Is4() {
		p := (*uint64)(unsafe.Add(unsafe.Pointer(&ip), 16))
		*p = z6noz
	}
	return ip
}

// PrefixTo6 maps an IPv4 prefix to an IPv4-mapped IPv6 prefix. It returns an
// IPv6 prefix unmodified.
func PrefixTo6(prefix netip.Prefix) netip.Prefix {
	if prefix.Addr().Is4() {
		return netip.PrefixFrom(AddrTo6(prefix.Addr()), prefix.Bits()+96)
	}
	return prefix
}

// UnmapPrefix unmaps a IPv4-mapped IPv6 prefix to IPv4 if it is one. Otherwise,
// it returns the provided prefix unmodified.
func UnmapPrefix(prefix netip.Prefix) netip.Prefix {
	if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
		ipv4Addr := prefix.Addr().Unmap()
		return netip.PrefixFrom(ipv4Addr, prefix.Bits()-96)
	}
	return prefix
}
