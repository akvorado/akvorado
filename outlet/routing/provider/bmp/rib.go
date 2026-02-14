// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"iter"
	"net/netip"
	"sync"
	"sync/atomic"
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
	tree          atomic.Pointer[bart.Table[prefixIndex]]
	routes        map[routeKey]route // map[routeKey]route where routeKey = (prefixIdx << 32) | routeIdx
	routesMu      sync.RWMutex       // protects routes map only
	nextPrefixID  prefixIndex        // counter for next prefix index (writer-only, protected by Provider.mu)
	freePrefixIDs []prefixIndex      // free list for reused prefix indices (writer-only, protected by Provider.mu)
}

// route contains the peer (external opaque value), the NLRI, the next
// hop and route attributes. The primary key is prefix (implied), peer
// and nlri. Fields are ordered to minimize padding: the two small
// fields (peer, prefixLen) share the first 8-byte slot.
type route struct {
	peer       uint32
	prefixLen  uint8
	nlri       unique.Handle[nlri]
	nextHop    unique.Handle[netip.Addr]
	attributes unique.Handle[routeAttributesComparable]
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
// The tree lookup is lock-free (atomic load). The routes map is briefly read-locked.
func (r *rib) IterateRoutes(ip netip.Addr) iter.Seq[route] {
	return func(yield func(route) bool) {
		prefixIdx, found := r.tree.Load().Lookup(ip.Unmap())
		if found {
			r.routesMu.RLock()
			defer r.routesMu.RUnlock()
			r.iterateRoutesForPrefixIndex(prefixIdx)(yield)
		}
	}
}

// AddPrefix add a new route to the RIB. It returns the number of routes really added.
// Two-phase: Phase 1 modifies routes map under routesMu, Phase 2 updates tree via Persist (no lock).
// Must be called under Provider.mu for writer serialization.
func (r *rib) AddPrefix(prefix netip.Prefix, newRoute route) int {
	prefix = helpers.UnmapPrefix(prefix)

	// Phase 1: modify routes map
	r.routesMu.Lock()
	tree := r.tree.Load()
	prefixIdx, found := tree.Get(prefix)
	newPrefix := !found
	if newPrefix {
		prefixIdx = r.newPrefixIndex()
	}

	// Check if route already exists (same peer and nlri)
	key := makeRouteKey(prefixIdx, 0)
	result := 0
	for {
		existingRoute, exists := r.routes[key]
		if !exists {
			// Found empty slot, add new route
			r.routes[key] = newRoute
			result = 1
			break
		}
		if existingRoute.peer == newRoute.peer && existingRoute.nlri == newRoute.nlri {
			// Found existing route, update it
			r.routes[key] = newRoute
			break
		}
		key++
	}
	r.routesMu.Unlock()

	// Phase 2: update tree if needed (no lock held)
	if newPrefix {
		newTree := tree.InsertPersist(prefix, prefixIdx)
		r.tree.Store(newTree)
	}

	return result
}

// RemovePrefix removes a route from the RIB. It returns the number of routes
// really removed. First, we modify routes map under routesMu, then we update
// the tree via *Persist methods (no lock). Must be called under Provider.mu for
// writer serialization.
func (r *rib) RemovePrefix(prefix netip.Prefix, oldRoute route) int {
	prefix = helpers.UnmapPrefix(prefix)

	// Phase 1: modify routes map
	r.routesMu.Lock()
	tree := r.tree.Load()
	prefixIdx, found := tree.Get(prefix)
	if !found {
		r.routesMu.Unlock()
		return 0
	}
	removedCount, empty := r.removeRoutes(prefixIdx, func(route route) bool {
		return route.peer == oldRoute.peer && route.nlri == oldRoute.nlri
	}, true)
	r.routesMu.Unlock()

	// Phase 2: update tree if prefix became empty (no lock held)
	if empty {
		newTree := tree.DeletePersist(prefix)
		r.tree.Store(newTree)
		r.freePrefixIndex(prefixIdx)
	}

	return removedCount
}

// FlushPeer removes a whole peer from the RIB, returning the number
// of removed routes.
// Two-phase: Phase 1 modifies routes map under routesMu, Phase 2 rebuilds tree (no lock).
// Must be called under Provider.mu for writer serialization.
func (r *rib) FlushPeer(peer uint32) int {
	// Phase 1: modify routes map, record empty prefixes
	r.routesMu.Lock()
	tree := r.tree.Load()
	removedTotal := 0
	anyEmpty := false
	emptyPrefixIDs := make(map[prefixIndex]struct{})

	for _, prefixIdx := range tree.All() {
		removed, empty := r.removeRoutes(prefixIdx, func(route route) bool {
			return route.peer == peer
		}, false)
		removedTotal += removed
		if empty {
			anyEmpty = true
			emptyPrefixIDs[prefixIdx] = struct{}{}
			r.freePrefixIndex(prefixIdx)
		}
	}
	r.routesMu.Unlock()

	// Phase 2: rebuild tree if any prefixes became empty (no lock held)
	if anyEmpty {
		// We need to rebuild the tree. A typical tree is 1M routes, this should
		// be pretty fast. Moreover, loosing a peer is not a condition happening
		// often.
		newTree := &bart.Table[prefixIndex]{}
		for prefix, prefixIdx := range tree.All() {
			if _, empty := emptyPrefixIDs[prefixIdx]; !empty {
				newTree.Insert(prefix, prefixIdx)
			}
		}
		r.tree.Store(newTree)
	}
	return removedTotal
}

// newRIB initializes a new RIB.
func newRIB() *rib {
	r := &rib{
		routes:        make(map[routeKey]route),
		nextPrefixID:  1, // Start from 1, 0 means to be removed
		freePrefixIDs: make([]prefixIndex, 0),
	}
	r.tree.Store(&bart.Table[prefixIndex]{})
	return r
}
