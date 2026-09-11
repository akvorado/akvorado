// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"net/netip"
	"unsafe"
)

// addrProxy has the same layout as netip.Addr. TestNetIPAddrStructure checks
// this is still true.
type addrProxy struct {
	addr [2]uint64      // netip.uint128
	z    unsafe.Pointer // unique.Handle[netip.addrDetail]
}

var (
	anyIPv6 = netip.IPv6Unspecified()
	z6noz   = (*addrProxy)(unsafe.Pointer(&anyIPv6)).z
)

// AddrTo6 maps an IPv4 address to an IPv4-mapped IPv6 address. It returns an
// IPv6 address unmodified. netip already stores an IPv4 address as
// ::ffff:a.b.c.d, so only the family marker has to change. This is unsafe, but
// there is a test to ensure netip.Addr is like we expect. Copying a
// unique.Handle bypasses the unique package bookkeeping, but z6noz lives for
// the whole program.
//
// This would be trivial to implement inside netip:
//
//	func (ip Addr) To6() Addr {
//		if ip.Is4() {
//			ip.z = z6noz
//		}
//		return ip
//	}
func AddrTo6(ip netip.Addr) netip.Addr {
	if !ip.Is4() {
		return ip
	}
	(*addrProxy)(unsafe.Pointer(&ip)).z = z6noz
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
