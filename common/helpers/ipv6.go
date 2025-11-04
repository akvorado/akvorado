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

// AddrTo6 maps an IPv4 to an IPv4-mapped IPv6 and returns an IPv6 unmodified.
// This is unsafe, but there is a test to ensure netip.Addr is like we expect.
// Copying a unique.Handle bypass reference count, but z6noz is "static".
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
