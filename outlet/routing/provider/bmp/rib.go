// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"iter"
	"net/netip"
	"sync"
	"unsafe"

	"akvorado/common/helpers"
	"akvorado/common/helpers/intern"

	"github.com/gaissmai/bart"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
)

// shardBits is the number of high bits used to encode the shard number.
const shardBits = 8

// prefixIndex is a typed index for prefixes in the RIB.
// The high shardBits encode the shard index, the remaining bits are the local ID.
type prefixIndex uint32

// routeIndex is a typed index for routes within a prefix
type routeIndex uint32

// routeKey is a typed key for route map entries
type routeKey uint64

// makePrefixIndex creates a global prefixIndex from shard index and local ID.
func makePrefixIndex(shardIdx int, localID prefixIndex) prefixIndex {
	return prefixIndex(uint32(shardIdx)<<(32-shardBits)) | localID
}

// shardIdx extracts the shard index from a global prefixIndex.
func (idx prefixIndex) shardIdx() int {
	return int(idx >> (32 - shardBits))
}

// ribShard holds the per-shard state for a RIB shard.
type ribShard struct {
	mu            sync.RWMutex
	idx           int
	routes        map[routeKey]route
	nlris         *intern.Pool[nlri]
	nextHops      *intern.Pool[nextHop]
	rtas          *intern.Pool[routeAttributes]
	nextPrefixID  prefixIndex   // counter for next prefix index
	freePrefixIDs []prefixIndex // free list for reused prefix indices
}

// rib represents the RIB.
type rib struct {
	mu     sync.RWMutex             // protects tree
	tree   *bart.Table[prefixIndex] // stores global prefix indices
	shards []*ribShard
}

// route contains the peer (external opaque value), the NLRI, the next
// hop and route attributes. The primary key is prefix (implied), peer
// and nlri. References are interned within a shard.
type route struct {
	peer       uint32
	nlri       intern.Reference[nlri]
	nextHop    intern.Reference[nextHop]
	attributes intern.Reference[routeAttributes]
	prefixLen  uint8
}

// rawRoute contains raw (non-interned) values for adding a route.
type rawRoute struct {
	peer       uint32
	nlri       nlri
	nextHop    nextHop
	attributes routeAttributes
	prefixLen  uint8
}

// nlri is the NLRI for the route (when combined with prefix). The
// route family is included as we may normalize NLRI accross AFI/SAFI.
type nlri struct {
	family bgp.Family
	path   uint32
	rd     RD
}

// Hash returns a hash for an NLRI
func (n nlri) Hash() uint64 {
	state := makeHash()
	state.Add((*byte)(unsafe.Pointer(&n.family)), int(unsafe.Sizeof(n.family)))
	state.Add((*byte)(unsafe.Pointer(&n.path)), int(unsafe.Sizeof(n.path)))
	state.Add((*byte)(unsafe.Pointer(&n.rd)), int(unsafe.Sizeof(n.rd)))
	return state.Sum()
}

// Equal tells if two NLRI are equal.
func (n nlri) Equal(n2 nlri) bool {
	return n == n2
}

// nextHop is just an IP address.
type nextHop netip.Addr

// Hash returns a hash for the next hop.
func (nh nextHop) Hash() uint64 {
	ip := netip.Addr(nh).As16()
	state := makeHash()
	state.Add((*byte)(unsafe.Pointer(&ip[0])), 16)
	return state.Sum()
}

// Equal tells if two next hops are equal.
func (nh nextHop) Equal(nh2 nextHop) bool {
	return nh == nh2
}

// routeAttributes is a set of route attributes.
type routeAttributes struct {
	asn         uint32
	asPath      []uint32
	communities []uint32
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
	if len(rta.largeCommunities) > 0 {
		// There is a test to check that this computation is
		// correct (the struct is 12-byte aligned, not
		// 16-byte).
		state.Add((*byte)(unsafe.Pointer(&rta.largeCommunities[0])), len(rta.largeCommunities)*int(unsafe.Sizeof(rta.largeCommunities[0])))
	}
	return state.Sum()
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

// shardForPrefix determines which shard a prefix belongs to using FNV-1a hash.
func (r *rib) shardForPrefix(prefix netip.Prefix) int {
	a := prefix.Addr().As16()
	h := uint32(2166136261)
	for _, b := range a {
		h ^= uint32(b)
		h *= 16777619
	}
	return int(h % uint32(len(r.shards)))
}

// newPrefixIndex allocates a new prefix index for this shard.
func (rs *ribShard) newPrefixIndex() prefixIndex {
	if len(rs.freePrefixIDs) > 0 {
		localID := rs.freePrefixIDs[len(rs.freePrefixIDs)-1]
		rs.freePrefixIDs = rs.freePrefixIDs[:len(rs.freePrefixIDs)-1]
		return makePrefixIndex(rs.idx, localID)
	}
	localID := rs.nextPrefixID
	rs.nextPrefixID++
	return makePrefixIndex(rs.idx, localID)
}

// freePrefixIndex returns a prefix ID to the shard's free list.
func (rs *ribShard) freePrefixIndex(globalID prefixIndex) {
	localID := globalID & ((1 << (32 - shardBits)) - 1)
	if helpers.Testing() {
		if localID == 0 {
			panic("cannot free index 0")
		}
	}
	rs.freePrefixIDs = append(rs.freePrefixIDs, localID)
}

// makeRouteKey creates a route key from prefix and route indices
func makeRouteKey(prefixIdx prefixIndex, routeIdx routeIndex) routeKey {
	return routeKey((uint64(prefixIdx) << 32) | uint64(routeIdx))
}

// iterateRoutesForPrefixIndex returns an iterator over all routes for a given prefix index
func (rs *ribShard) iterateRoutesForPrefixIndex(prefixIdx prefixIndex) iter.Seq[route] {
	return func(yield func(route) bool) {
		key := makeRouteKey(prefixIdx, 0)
		for {
			route, exists := rs.routes[key]
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
func (rs *ribShard) removeRoutes(prefixIdx prefixIndex, shouldRemove func(route) bool, once bool) (int, bool) {
	removed := 0
	checkKey := makeRouteKey(prefixIdx, 0)
	nextKey := checkKey
	skip := false // skip remaining routes if once is true

	for {
		existingRoute, exists := rs.routes[checkKey]
		if !exists {
			break
		}

		if !skip && shouldRemove(existingRoute) {
			// Remove this route
			rs.nlris.Take(existingRoute.nlri)
			rs.nextHops.Take(existingRoute.nextHop)
			rs.rtas.Take(existingRoute.attributes)
			delete(rs.routes, checkKey)
			removed++
			if once {
				skip = true
			}
		} else {
			// Keep this route, move it to nextKey if needed
			if checkKey != nextKey {
				rs.routes[nextKey] = existingRoute
				delete(rs.routes, checkKey)
			}
			nextKey++ // advance to next position for kept routes
		}
		checkKey++ // always advance check position
	}

	return removed, nextKey == makeRouteKey(prefixIdx, 0)
}

// AddRoute adds a new route to the RIB. It returns the number of routes
// really added and whether the prefix is new in the tree.
func (r *rib) AddRoute(prefix netip.Prefix, rr rawRoute) (int, bool) {
	prefix = helpers.UnmapPrefix(prefix)
	shardIdx := r.shardForPrefix(prefix)
	rs := r.shards[shardIdx]

	rs.mu.Lock()
	defer rs.mu.Unlock()

	nlriRef := rs.nlris.Put(rr.nlri)
	nhRef := rs.nextHops.Put(rr.nextHop)
	rtaRef := rs.rtas.Put(rr.attributes)
	newRoute := route{
		peer:       rr.peer,
		nlri:       nlriRef,
		nextHop:    nhRef,
		attributes: rtaRef,
		prefixLen:  rr.prefixLen,
	}

	var prefixIdx prefixIndex
	var isNew bool
	r.mu.Lock()
	r.tree.Modify(prefix, func(existing prefixIndex, found bool) (prefixIndex, bool) {
		if found {
			prefixIdx = existing
		} else {
			isNew = true
			prefixIdx = rs.newPrefixIndex()
		}
		return prefixIdx, false
	})
	r.mu.Unlock()

	// Check if route already exists (same peer and nlri)
	key := makeRouteKey(prefixIdx, 0)
	for {
		existingRoute, exists := rs.routes[key]
		if !exists {
			// Found empty slot, add new route
			rs.routes[key] = newRoute
			return 1, isNew
		}
		if existingRoute.peer == newRoute.peer && existingRoute.nlri == newRoute.nlri {
			// Found existing route, update it
			rs.nlris.Take(existingRoute.nlri)
			rs.nextHops.Take(existingRoute.nextHop)
			rs.rtas.Take(existingRoute.attributes)
			rs.routes[key] = newRoute
			return 0, false // Not really added, just updated
		}
		key++
	}
}

// RemoveRoute removes a route from the RIB. It returns the number of routes
// really removed and whether the prefix was removed from the tree.
func (r *rib) RemoveRoute(prefix netip.Prefix, rr rawRoute) (int, bool) {
	prefix = helpers.UnmapPrefix(prefix)
	shardIdx := r.shardForPrefix(prefix)
	rs := r.shards[shardIdx]

	rs.mu.Lock()
	defer rs.mu.Unlock()

	nlriRef, ok := rs.nlris.Ref(rr.nlri)
	if !ok {
		return 0, false
	}

	removedCount := 0
	prefixRemoved := false

	r.mu.Lock()
	r.tree.Modify(prefix, func(existing prefixIndex, found bool) (prefixIndex, bool) {
		if found {
			var empty bool
			removedCount, empty = rs.removeRoutes(existing, func(route route) bool {
				return route.peer == rr.peer && route.nlri == nlriRef
			}, true)
			if empty {
				rs.freePrefixIndex(existing)
				prefixRemoved = true
				return 0, true
			}
			return existing, false
		}
		return 0, true
	})
	r.mu.Unlock()

	return removedCount, prefixRemoved
}

// FlushPeer removes a whole peer from the RIB, returning the number
// of removed routes and the number of removed prefixes.
func (r *rib) FlushPeer(peer uint32) (int, int) {
	routesRemoved := 0
	prefixesRemoved := 0
	anyEmpty := false

	// Lock all shards in order, then tree
	for _, rs := range r.shards {
		rs.mu.Lock()
	}
	r.mu.Lock()

	// Iterate through all prefixes and remove peer routes.
	for _, prefixIdx := range r.tree.All() {
		rs := r.shards[prefixIdx.shardIdx()]
		removed, empty := rs.removeRoutes(prefixIdx, func(route route) bool {
			return route.peer == peer
		}, false)
		routesRemoved += removed
		if empty {
			rs.freePrefixIndex(prefixIdx)
			prefixesRemoved++
		}
		anyEmpty = anyEmpty || empty
	}

	if anyEmpty {
		// Rebuild the tree excluding empty prefixes.
		newTree := &bart.Table[prefixIndex]{}
		for prefix, prefixIdx := range r.tree.All() {
			if _, hasRoutes := r.shards[prefixIdx.shardIdx()].routes[makeRouteKey(prefixIdx, 0)]; hasRoutes {
				newTree.Insert(prefix, prefixIdx)
			}
		}
		r.tree = newTree
	}

	r.mu.Unlock()
	for i := len(r.shards) - 1; i >= 0; i-- {
		r.shards[i].mu.Unlock()
	}

	return routesRemoved, prefixesRemoved
}

// LookupRoute looks up the best matching route for an IP address,
// preferring routes with the given next hop. It returns route attributes,
// next hop, prefix length, and whether a route was found.
func (r *rib) LookupRoute(ip, preferredNH netip.Addr) (routeAttributes, nextHop, uint8, bool) {
	r.mu.RLock()
	prefixIdx, found := r.tree.Lookup(ip.Unmap())
	r.mu.RUnlock()

	if !found {
		return routeAttributes{}, nextHop{}, 0, false
	}

	rs := r.shards[prefixIdx.shardIdx()]

	rs.mu.RLock()
	defer rs.mu.RUnlock()

	var selectedRoute route
	routeFound := false
	for route := range rs.iterateRoutesForPrefixIndex(prefixIdx) {
		if !routeFound {
			selectedRoute = route
			routeFound = true
		}
		if rs.nextHops.Get(route.nextHop) == nextHop(preferredNH) {
			selectedRoute = route
			break
		}
	}

	if !routeFound {
		return routeAttributes{}, nextHop{}, 0, false
	}

	return rs.rtas.Get(selectedRoute.attributes),
		rs.nextHops.Get(selectedRoute.nextHop),
		selectedRoute.prefixLen, true
}

// newRIB initializes a new RIB with the specified number of shards.
func newRIB(nShards int) *rib {
	shards := make([]*ribShard, nShards)
	for i := range nShards {
		shards[i] = &ribShard{
			idx:           i,
			routes:        make(map[routeKey]route),
			nlris:         intern.NewPool[nlri](),
			nextHops:      intern.NewPool[nextHop](),
			rtas:          intern.NewPool[routeAttributes](),
			nextPrefixID:  1, // Start from 1, 0 means to be removed
			freePrefixIDs: make([]prefixIndex, 0),
		}
	}
	return &rib{
		tree:   &bart.Table[prefixIndex]{},
		shards: shards,
	}
}
