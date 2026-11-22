// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flowmodel

import (
	"crypto/rand"
	"fmt"
	"net/netip"
)

// AnonymizePrefix returns a random prefix inside target with the same length as
// p. Nothing links the result to p, so the caller has to retry on collisions.
func AnonymizePrefix(p, target netip.Prefix) (netip.Prefix, error) {
	if p.Addr().Is4() != target.Addr().Is4() {
		return netip.Prefix{}, fmt.Errorf("prefix %s and target %s have different address families",
			p, target)
	}
	if target.Bits() >= p.Bits() {
		return netip.Prefix{}, fmt.Errorf("target %s is not shorter than prefix %s", target, p)
	}

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return netip.Prefix{}, err
	}
	result := target.Masked().Addr().AsSlice()
	for i := target.Bits(); i < p.Bits(); i++ {
		setBit(result, i, getBit(random, i))
	}
	addr, ok := netip.AddrFromSlice(result)
	if !ok {
		return netip.Prefix{}, fmt.Errorf("cannot build address from %v", result)
	}
	return netip.PrefixFrom(addr, p.Bits()), nil
}

// getBit returns the bit at the given position, most significant first.
func getBit(data []byte, position int) byte {
	return (data[position/8] >> (7 - position%8)) & 1
}

// setBit sets the bit at the given position, most significant first. The bit
// must be zero before the call.
func setBit(data []byte, position int, value byte) {
	data[position/8] |= value << (7 - position%8)
}
