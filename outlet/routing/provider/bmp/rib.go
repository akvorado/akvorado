// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"iter"
	"net/netip"
	"unique"
	"unsafe"

	"akvorado/common/helpers"

	"github.com/gaissmai/bart"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
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
	nextPrefixID  prefixIndex              // counter for next prefix index
	freePrefixIDs []prefixIndex            // free list for reused prefix indices
}

// route contains the peer (external opaque value), the NLRI, the next
// hop and route attributes. The primary key is prefix (implied), peer
// and nlri.
type route struct {
	peer       uint32
	nlri       unique.Handle[nlri]
	nextHop    unique.Handle[netip.Addr]
	attributes unique.Handle[routeAttributesComparable]
	prefixLen  uint8
}

// nlri is the NLRI for the route (when combined with prefix). The
// route family is included as we may normalize NLRI accross AFI/SAFI.
type nlri struct {
	family bgp.Family
	path   uint32
	rd     RD
}

// routeAttributes is a set of route attributes with natural Go types.
type routeAttributes struct {
	asn              uint32
	asPath           []uint32
	communities      []uint32
	largeCommunities []bgp.LargeCommunity
}

// routeAttributesComparable is the comparable (internable) encoding
// of routeAttributes. Slice fields are binary-encoded as strings
// using unsafe to make the struct comparable for use with
// unique.Handle. Each string is created zero-copy from the source
// slice; unique.Make clones string data only when inserting a new entry.
type routeAttributesComparable struct {
	asn              uint32
	asPath           string // binary-encoded []uint32
	communities      string // binary-encoded []uint32
	largeCommunities string // binary-encoded []bgp.LargeCommunity
}

// ToComparable converts routeAttributes to its comparable form for
// use with unique.Make.
func (rta routeAttributes) ToComparable() routeAttributesComparable {
	return routeAttributesComparable{
		asn:              rta.asn,
		asPath:           uint32sToString(rta.asPath),
		communities:      uint32sToString(rta.communities),
		largeCommunities: largeCommunitiesToString(rta.largeCommunities),
	}
}

func uint32sToString(s []uint32) string {
	if len(s) == 0 {
		return ""
	}
	return unsafe.String((*byte)(unsafe.Pointer(&s[0])), len(s)*4)
}

func stringToUint32s(s string) []uint32 {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.StringData(s))), len(s)/4)
}

func largeCommunitiesToString(s []bgp.LargeCommunity) string {
	if len(s) == 0 {
		return ""
	}
	return unsafe.String((*byte)(unsafe.Pointer(&s[0])), len(s)*12)
}

func stringToLargeCommunities(s string) []bgp.LargeCommunity {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice((*bgp.LargeCommunity)(unsafe.Pointer(unsafe.StringData(s))), len(s)/12)
}

// getASPath returns the AS path as a slice of uint32.
func (rta routeAttributesComparable) getASPath() []uint32 {
	return stringToUint32s(rta.asPath)
}

// getCommunities returns the communities as a slice of uint32.
func (rta routeAttributesComparable) getCommunities() []uint32 {
	return stringToUint32s(rta.communities)
}

// getLargeCommunities returns the large communities as a slice of bgp.LargeCommunity.
func (rta routeAttributesComparable) getLargeCommunities() []bgp.LargeCommunity {
	return stringToLargeCommunities(rta.largeCommunities)
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
	if helpers.Testing() {
		if id == 0 {
			panic("cannot free index 0")
		}
	}
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

// IterateRoutes will iterate on all the routes matching the provided IP address.
func (r *rib) IterateRoutes(ip netip.Addr) iter.Seq[route] {
	return func(yield func(route) bool) {
		prefixIdx, found := r.tree.Lookup(ip.Unmap())
		if found {
			r.iterateRoutesForPrefixIndex(prefixIdx)(yield)
		}
	}
}

// AddPrefix add a new route to the RIB. It returns the number of routes really added.
func (r *rib) AddPrefix(prefix netip.Prefix, newRoute route) int {
	var prefixIdx prefixIndex
	prefix = helpers.UnmapPrefix(prefix)
	r.tree.Modify(prefix, func(existing prefixIndex, found bool) (prefixIndex, bool) {
		if found {
			prefixIdx = existing
		} else {
			prefixIdx = r.newPrefixIndex()
		}
		return prefixIdx, false
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
			r.routes[key] = newRoute
			return 0 // Not really added, just updated
		}
		key++
	}
}

// RemovePrefix removes a route from the RIB. It returns the number of routes really removed.
func (r *rib) RemovePrefix(prefix netip.Prefix, oldRoute route) int {
	removedCount := 0
	prefix = helpers.UnmapPrefix(prefix)

	r.tree.Modify(prefix, func(existing prefixIndex, found bool) (prefixIndex, bool) {
		if found {
			var empty bool
			removedCount, empty = r.removeRoutes(existing, func(route route) bool {
				return route.peer == oldRoute.peer && route.nlri == oldRoute.nlri
			}, true)
			if empty {
				r.freePrefixIndex(existing)
				return 0, true
			}
			return existing, false
		}
		return 0, true
	})

	return removedCount
}

// FlushPeer removes a whole peer from the RIB, returning the number
// of removed routes.
func (r *rib) FlushPeer(peer uint32) int {
	removedTotal := 0
	anyEmpty := false

	// Iterate through all prefixes and remove peer routes.
	for _, prefixIdx := range r.tree.All() {
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
		for prefix, prefixIdx := range r.tree.All() {
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
		nextPrefixID:  1, // Start from 1, 0 means to be removed
		freePrefixIDs: make([]prefixIndex, 0),
	}
}
