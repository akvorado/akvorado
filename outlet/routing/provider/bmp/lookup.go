// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"context"
	"errors"
	"net/netip"

	"akvorado/outlet/routing/provider"
)

// LookupResult is the result of the Lookup() function.
type LookupResult = provider.LookupResult

var errNoRouteFound = errors.New("no route found")

// Lookup lookups a route for the provided IP address. It favors the
// provided next hop if provided. This is somewhat approximate because
// we use the best route we have, while the exporter may not have this
// best route available. The returned result should not be modified!
// The last parameter, the agent, is ignored by this provider.
func (p *Provider) Lookup(_ context.Context, ip netip.Addr, nh netip.Addr, _ netip.Addr) (LookupResult, error) {
	if !p.config.CollectASNs && !p.config.CollectASPaths && !p.config.CollectCommunities {
		return LookupResult{}, nil
	}
	if !p.active.Load() {
		return LookupResult{}, nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Find the most specific prefix for this IP
	prefixIdx, found := p.rib.tree.Lookup(ip)
	if !found {
		return LookupResult{}, errNoRouteFound
	}

	// Find the best route, preferring exact next hop match
	var selectedRoute route
	routeFound := false

	for route := range p.rib.iterateRoutesForPrefixIndex(prefixIdx) {
		if p.rib.nextHops.Get(route.nextHop) == nextHop(nh) {
			// Exact match found, use it and don't search further
			selectedRoute = route
			break
		}
		// If we don't have a match already, use this one.
		if !routeFound {
			selectedRoute = route
			routeFound = true
		}
	}

	if !routeFound {
		return LookupResult{}, errNoRouteFound
	}

	attributes := p.rib.rtas.Get(selectedRoute.attributes)
	// The next hop is updated from the rib in every case, because the user
	// "opted in" for bmp as source if the lookup result is evaluated
	nh = netip.Addr(p.rib.nextHops.Get(selectedRoute.nextHop))

	// Prefix len is v6 coded in the bmp rib. We need to substract 96 if it's a v4 prefix
	plen := selectedRoute.prefixLen
	if ip.Is4In6() {
		plen = plen - 96
	}
	return LookupResult{
		ASN:              attributes.asn,
		ASPath:           attributes.asPath,
		Communities:      attributes.communities,
		LargeCommunities: attributes.largeCommunities,
		NetMask:          plen,
		NextHop:          nh,
	}, nil
}
