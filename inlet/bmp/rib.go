// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"context"
	"net/netip"
	"runtime"
	"sync/atomic"
	"unsafe"

	"akvorado/common/helpers"

	"github.com/kentik/patricia"
	tree "github.com/kentik/patricia/generics_tree"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
)

// rib represents the RIB.
type rib struct {
	tree     *tree.TreeV6[route]
	nextHops *helpers.InternPool[nextHop]
	rtas     *helpers.InternPool[routeAttributes]
}

// route contains the peer (external opaque value), the NLRI, the next hop and
// route attributes. The primary key is prefix (implied), peer and nlri. This
// structure is used a lot. To minimize its size, we order it carefully.
type route struct {
	nlri       nlri                                     // 64+32+32
	peer       uint32                                   // 32
	nextHop    helpers.InternReference[nextHop]         // 32
	attributes helpers.InternReference[routeAttributes] // 32
}

// nlri is the NLRI for the route (when combined with prefix). The
// route family is included as we may normalize NLRI accross AFI/SAFI.
type nlri struct {
	rd     RD          // 64
	family routeFamily // 32
	path   uint32      // 32
}

// routeFamily should be bgp.RouteFamily, but to be more memory-efficient, we use uint32
type routeFamily uint32

// nextHop is just an IP address.
type nextHop netip.Addr

// Hash returns a hash for the next hop.
func (nh nextHop) Hash() uint64 {
	ip := netip.Addr(nh).As16()
	state := rtaHashSeed
	return rthash((*byte)(unsafe.Pointer(&ip[0])), 16, state)
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
	state := rtaHashSeed
	state = rthash((*byte)(unsafe.Pointer(&rta.asn)), 4, state)
	if len(rta.asPath) > 0 {
		state = rthash((*byte)(unsafe.Pointer(&rta.asPath[0])), len(rta.asPath)*4, state)
	}
	if len(rta.communities) > 0 {
		state = rthash((*byte)(unsafe.Pointer(&rta.communities[0])), len(rta.communities)*4, state)
	}
	if len(rta.largeCommunities) > 0 {
		// There is a test to check that this computation is
		// correct (the struct is 12-byte aligned, not
		// 16-byte).
		state = rthash((*byte)(unsafe.Pointer(&rta.largeCommunities[0])), len(rta.largeCommunities)*12, state)
	}
	return state & rtaHashMask
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

// addPrefix add a new route to the RIB. It returns the number of routes really added.
func (r *rib) addPrefix(ip netip.Addr, bits int, new route) int {
	v6 := patricia.NewIPv6Address(ip.AsSlice(), uint(bits))
	added, _ := r.tree.AddOrUpdate(v6, new,
		func(r1, r2 route) bool {
			return r1.peer == r2.peer && r1.nlri == r2.nlri
		}, func(old route) route {
			r.nextHops.Take(old.nextHop)
			r.rtas.Take(old.attributes)
			return new
		})
	if !added {
		return 0
	}
	return 1
}

// removePrefix removes a route from the RIB. It returns the number of routes really removed.
func (r *rib) removePrefix(ip netip.Addr, bits int, old route) int {
	v6 := patricia.NewIPv6Address(ip.AsSlice(), uint(bits))
	removed := r.tree.Delete(v6, func(r1, r2 route) bool {
		// This is not enforced/documented, but the route in the tree is the first one.
		if r1.peer == r2.peer && r1.nlri == r2.nlri {
			r.nextHops.Take(old.nextHop)
			r.rtas.Take(r1.attributes)
			return true
		}
		return false
	}, old)
	return removed
}

// flushPeer removes a whole peer from the RIB, returning the number
// of removed routes.
func (r *rib) flushPeer(ctx context.Context, peer uint32, min int) (int, bool) {
	// Handle context done state
	done := atomic.Bool{}       // signal for the loop to stop when true
	stop := make(chan struct{}) // signal for the goroutine to stop
	defer close(stop)
	go func() {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			done.Store(true)
		}
	}()

	// Flush routes
	removed := 0
	buf := make([]route, 0)
	iter := r.tree.Iterate()
	runtime.Gosched()
	for iter.Next() {
		removed += iter.DeleteWithBuffer(buf, func(payload route, val route) bool {
			if payload.peer == peer {
				r.nextHops.Take(payload.nextHop)
				r.rtas.Take(payload.attributes)
				return true
			}
			return false
		}, route{})
		if removed >= min {
			if done.Load() {
				return removed, false
			}
		}
	}
	return removed, true
}

// newRIB initializes a new RIB.
func newRIB() *rib {
	return &rib{
		tree:     tree.NewTreeV6[route](),
		nextHops: helpers.NewInternPool[nextHop](),
		rtas:     helpers.NewInternPool[routeAttributes](),
	}
}
