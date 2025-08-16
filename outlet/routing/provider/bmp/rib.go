// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"iter"
	"net/netip"
	"unique"
	"unsafe"

	"akvorado/common/helpers/intern"

	"github.com/gaissmai/bart"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
)

// prefixIndex is a typed index for prefixes in the RIB
type prefixIndex uint32

// routeIndex is a typed index for routes within a prefix
type routeIndex uint32

// routeKey is a typed key for route map entries
type routeKey uint64

// rib represents the RIB.
type rib struct {
	tree          *bart.Table[prefixIndex] // stores prefix indices
	routes        map[routeKey]route       // map[routeKey]route where routeKey = (prefixIdx << 32) | routeIdx
	rtas          *intern.Pool[routeAttributes]
	nextPrefixID  prefixIndex   // counter for next prefix index
	freePrefixIDs []prefixIndex // free list for reused prefix indices
}

// route contains the peer (external opaque value), the NLRI, the next
// hop and route attributes. The primary key is prefix (implied), peer
// and nlri.
type route struct {
	peer       uint32
	nlri       unique.Handle[nlri]
	nextHop    unique.Handle[nextHop]
	attributes intern.Reference[routeAttributes]
}

// nlri is the NLRI for the route (when combined with prefix). The
// route family is included as we may normalize NLRI accross AFI/SAFI.
type nlri struct {
	family bgp.RouteFamily
	path   uint32
	rd     RD
}

// nextHop is just an IP address.
type nextHop netip.Addr

// routeAttributes is a set of route attributes.
type routeAttributes struct {
	asn         uint32
	asPath      []uint32
	communities []uint32
	plen        uint8
	// extendedCommunities []uint64
	largeCommunities []bgp.LargeCommunity
}

// Hash returns a hash for route attributes. This may seem like black
// magic, but this is important for performance.
func (rta routeAttributes) Hash() uint64 {
	state := makeHash()
	state.Add((*byte)(unsafe.Pointer(&rta.asn)), int(unsafe.Sizeof(rta.asn)))
	if len(rta.asPath) > 0 {
		state.Add((*byte)(unsafe.Pointer(&rta.asPath[0])), len(rta.asPath)*int(unsafe.Sizeof(rta.asPath[0])))
	}
	if len(rta.communities) > 0 {
		state.Add((*byte)(unsafe.Pointer(&rta.communities[0])), len(rta.communities)*int(unsafe.Sizeof(rta.communities[0])))
	}
	state.Add((*byte)(unsafe.Pointer(&rta.plen)), 1)
	if len(rta.largeCommunities) > 0 {
		// There is a test to check that this computation is
		// correct (the struct is 12-byte aligned, not
		// 16-byte).
		state.Add((*byte)(unsafe.Pointer(&rta.largeCommunities[0])), len(rta.largeCommunities)*int(unsafe.Sizeof(rta.largeCommunities[0])))
	}
	return state.Sum() & rtaHashMask
}

// Equal tells if two route attributes are equal.
func (rta routeAttributes) Equal(orta routeAttributes) bool {
	if rta.asn != orta.asn {
		return false
	}
	if len(rta.asPath) != len(orta.asPath) {
		return false
	}
	if len(rta.communities) != len(orta.communities) {
		return false
	}
	if rta.plen != orta.plen {
		return false
	}
	if len(rta.largeCommunities) != len(orta.largeCommunities) {
		return false
	}
	for idx := range rta.asPath {
		if rta.asPath[idx] != orta.asPath[idx] {
			return false
		}
	}
	for idx := range rta.communities {
		if rta.communities[idx] != orta.communities[idx] {
			return false
		}
	}
	for idx := range rta.largeCommunities {
		if rta.largeCommunities[idx] != orta.largeCommunities[idx] {
			return false
		}
	}
	return true
}

// newPrefixIndex allocates a new prefix index, reusing from free list if available
func (r *rib) newPrefixIndex() prefixIndex {
	if len(r.freePrefixIDs) > 0 {
		id := r.freePrefixIDs[len(r.freePrefixIDs)-1]
		r.freePrefixIDs = r.freePrefixIDs[:len(r.freePrefixIDs)-1]
		return id
	}
	id := r.nextPrefixID
	r.nextPrefixID++
	return id
}

// freePrefixIndex returns a prefix ID to the free list
func (r *rib) freePrefixIndex(id prefixIndex) {
	r.freePrefixIDs = append(r.freePrefixIDs, id)
}

// makeRouteKey creates a route key from prefix and route indices
func makeRouteKey(prefixIdx prefixIndex, routeIdx routeIndex) routeKey {
	return routeKey((uint64(prefixIdx) << 32) | uint64(routeIdx))
}

// iterateRoutesForPrefix returns an iterator over all routes for a given prefix index
func (r *rib) iterateRoutesForPrefixIndex(prefixIdx prefixIndex) iter.Seq[route] {
	return func(yield func(route) bool) {
		key := makeRouteKey(prefixIdx, 0)
		for {
			route, exists := r.routes[key]
			if !exists {
				break
			}
			if !yield(route) {
				break
			}
			key++
		}
	}
}

// removeRoutes removes routes matching the predicate and compacts remaining
// routes. If once is true, stops after removing the first matching route. If
// prefix becomes empty, removes it from the tree and frees the prefix ID. It
// returns the number of routes removed and a boolean to say if the prefix
// should be removed from the tree.
func (r *rib) removeRoutes(prefixIdx prefixIndex, shouldRemove func(route) bool, once bool) (int, bool) {
	removed := 0
	checkKey := makeRouteKey(prefixIdx, 0)
	nextKey := checkKey
	skip := false // skip remaining routes if once is true

	for {
		existingRoute, exists := r.routes[checkKey]
		if !exists {
			break
		}

		if !skip && shouldRemove(existingRoute) {
			// Remove this route
			r.rtas.Take(existingRoute.attributes)
			delete(r.routes, checkKey)
			removed++
			if once {
				skip = true
			}
		} else {
			// Keep this route, move it to nextKey if needed
			if checkKey != nextKey {
				r.routes[nextKey] = existingRoute
				delete(r.routes, checkKey)
			}
			nextKey++ // advance to next position for kept routes
		}
		checkKey++ // always advance check position
	}

	return removed, nextKey == makeRouteKey(prefixIdx, 0)
}

// addPrefix add a new route to the RIB. It returns the number of routes really added.
func (r *rib) addPrefix(prefix netip.Prefix, newRoute route) int {
	var prefixIdx prefixIndex
	r.tree.Update(prefix, func(existing prefixIndex, found bool) prefixIndex {
		if found {
			prefixIdx = existing
		} else {
			prefixIdx = r.newPrefixIndex()
		}
		return prefixIdx
	})

	// Check if route already exists (same peer and nlri)
	key := makeRouteKey(prefixIdx, 0)
	for {
		existingRoute, exists := r.routes[key]
		if !exists {
			// Found empty slot, add new route
			r.routes[key] = newRoute
			return 1
		}
		if existingRoute.peer == newRoute.peer && existingRoute.nlri == newRoute.nlri {
			// Found existing route, update it
			r.rtas.Take(existingRoute.attributes)
			r.routes[key] = newRoute
			return 0 // Not really added, just updated
		}
		key++
	}
}

// removePrefix removes a route from the RIB. It returns the number of routes really removed.
func (r *rib) removePrefix(prefix netip.Prefix, oldRoute route) int {
	removedCount := 0
	empty := false

	// Use Update to access prefix and remove route
	r.tree.Update(prefix, func(existing prefixIndex, found bool) prefixIndex {
		if found {
			removedCount, empty = r.removeRoutes(existing, func(route route) bool {
				return route.peer == oldRoute.peer && route.nlri == oldRoute.nlri
			}, true)
			if empty {
				r.freePrefixIndex(existing)
				return 0
			}
			return existing
		}
		// We use Update() to avoid to do a double lookup because in most cases,
		// we will remove prefix that exists. However, it is also valid to
		// remove a prefix that does not exist. In this case, Update() created a
		// node while we don't need it. We remove it, making this operation costly.
		empty = true
		return 0
	})

	if empty {
		r.tree.Delete(prefix)
	}
	return removedCount
}

// flushPeer removes a whole peer from the RIB, returning the number
// of removed routes.
func (r *rib) flushPeer(peer uint32) int {
	removedTotal := 0
	anyEmpty := false

	// Iterate through all prefixes and remove peer routes.
	for _, prefixIdx := range r.tree.All6() {
		removed, empty := r.removeRoutes(prefixIdx, func(route route) bool {
			return route.peer == peer
		}, false)
		removedTotal += removed
		anyEmpty = anyEmpty || empty
	}

	if anyEmpty {
		// We need to rebuild the tree. A typical tree is 1M routes, this should
		// be pretty fast. Moreover, loosing a peer is not a condition happening
		// often.
		newTree := &bart.Table[prefixIndex]{}
		for prefix, prefixIdx := range r.tree.All6() {
			if prefixIdx != 0 {
				newTree.Insert(prefix, prefixIdx)
			}
		}
		r.tree = newTree
	}
	return removedTotal
}

// newRIB initializes a new RIB.
func newRIB() *rib {
	return &rib{
		tree:          &bart.Table[prefixIndex]{},
		routes:        make(map[routeKey]route),
		rtas:          intern.NewPool[routeAttributes](),
		nextPrefixID:  1, // Start from 1, 0 means to be removed
		freePrefixIDs: make([]prefixIndex, 0),
	}
}
