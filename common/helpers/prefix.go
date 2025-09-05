// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import "net/netip"

// UnmapPrefix unmaps a IPv4-mapped IPv6 prefix to IPv4 if it is one. Otherwise,
// it returns the provided prefix unmodified.
func UnmapPrefix(prefix netip.Prefix) netip.Prefix {
	if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
		ipv4Addr := prefix.Addr().Unmap()
		return netip.PrefixFrom(ipv4Addr, prefix.Bits()-96)
	}
	return prefix
}
