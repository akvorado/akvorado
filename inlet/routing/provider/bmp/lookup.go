// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"context"
	"net/netip"

	"github.com/kentik/patricia"

	"akvorado/inlet/routing/provider"
)

// LookupResult is the result of the Lookup() function.
type LookupResult = provider.LookupResult

// Lookup lookups a route for the provided IP address. It favors the
// provided next hop if provided. This is somewhat approximate because
// we use the best route we have, while the exporter may not have this
// best route available. The returned result should not be modified!
func (p *Provider) Lookup(_ context.Context, ip netip.Addr, nh netip.Addr) LookupResult {
	if !p.config.CollectASNs && !p.config.CollectASPaths && !p.config.CollectCommunities {
		return LookupResult{}
	}
	v6 := patricia.NewIPv6Address(ip.AsSlice(), 128)

	p.mu.RLock()
	defer p.mu.RUnlock()

	bestFound := false
	found := false
	_, routes := p.rib.tree.FindDeepestTagsWithFilter(v6, func(route route) bool {
		if bestFound {
			// We already have the best route, skip remaining routes
			return false
		}
		if p.rib.nextHops.Get(route.nextHop) == nextHop(nh) {
			// Exact match found, use it and don't search further
			bestFound = true
			return true
		}
		// If we don't have a match already, use this one.
		if !found {
			found = true
			return true
		}
		// Otherwise, skip it
		return false
	})
	if len(routes) == 0 {
		return LookupResult{}
	}
	attributes := p.rib.rtas.Get(routes[len(routes)-1].attributes)
	// prefix len is v6 coded in the bmp rib. We need to substract 96 if it's a v4 prefix
	plen := attributes.plen
	if ip.Is4() || ip.Is4In6() {
		plen = plen - 96
	}
	return LookupResult{
		ASN:              attributes.asn,
		ASPath:           attributes.asPath,
		Communities:      attributes.communities,
		LargeCommunities: attributes.largeCommunities,
		NetMask:          plen,
	}
}
