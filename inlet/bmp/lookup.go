// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"net"
	"net/netip"

	"github.com/kentik/patricia"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
)

// LookupResult is the result of the Lookup() function.
type LookupResult struct {
	ASN              uint32
	ASPath           []uint32
	Communities      []uint32
	LargeCommunities []bgp.LargeCommunity
}

// Lookup lookups a route for the provided IP address. It favors the
// provided next hop if provided. This is somewhat approximate because
// we use the best route we have, while the exporter may not have this
// best route available. The returned result should not be modified!
func (c *Component) Lookup(addrIP net.IP, nextHopIP net.IP) (result LookupResult) {
	if !c.config.CollectASNs && !c.config.CollectASPaths && !c.config.CollectCommunities {
		return
	}
	ip, _ := netip.AddrFromSlice(addrIP.To16())
	nh, _ := netip.AddrFromSlice(nextHopIP.To16())
	v6 := patricia.NewIPv6Address(ip.AsSlice(), 128)

	lookup := func(rib *rib) error {
		bestFound := false
		found := false
		_, routes := rib.tree.FindDeepestTagsWithFilter(v6, func(route route) bool {
			if bestFound {
				// We already have the best route, skip remaining routes
				return false
			}
			if rib.nextHops.Get(route.nextHop) == nextHop(nh) {
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
			return nil
		}
		attributes := rib.rtas.Get(routes[len(routes)-1].attributes)
		result = LookupResult{
			ASN:              attributes.asn,
			ASPath:           attributes.asPath,
			Communities:      attributes.communities,
			LargeCommunities: attributes.largeCommunities,
		}
		return nil
	}

	switch c.config.RIBMode {
	case RIBModeMemory:
		c.ribWorkerQueue(func(s *ribWorkerState) error {
			return lookup(s.rib)
		}, ribWorkerHighPriority)
	case RIBModePerformance:
		lookup(c.ribReadonly.Load())
	}
	return
}
