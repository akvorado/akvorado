// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"context"
	"net/netip"
	"runtime"
	"sync/atomic"
	"unsafe"

	"akvorado/common/helpers/intern"

	"github.com/kentik/patricia"
	tree "github.com/kentik/patricia/generics_tree"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
)

// rib represents the RIB.
type rib struct {
	tree     *tree.TreeV6[route]
	nlris    *intern.Pool[nlri]
	nextHops *intern.Pool[nextHop]
	rtas     *intern.Pool[routeAttributes]
}

// route contains the peer (external opaque value), the NLRI, the next
// hop and route attributes. The primary key is prefix (implied), peer
// and nlri.
type route struct {
	peer       uint32
	nlri       intern.Reference[nlri]
	nextHop    intern.Reference[nextHop]
	attributes intern.Reference[routeAttributes]
}

// nlri is the NLRI for the route (when combined with prefix). The
// route family is included as we may normalize NLRI accross AFI/SAFI.
type nlri struct {
	family bgp.RouteFamily
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
func (r *rib) addPrefix(ip netip.Addr, bits int, newRoute route) int {
	v6 := patricia.NewIPv6Address(ip.AsSlice(), uint(bits))
	added, _ := r.tree.AddOrUpdate(v6, newRoute,
		func(r1, r2 route) bool {
			return r1.peer == r2.peer && r1.nlri == r2.nlri
		}, func(old route) route {
			r.nlris.Take(old.nlri)
			r.nextHops.Take(old.nextHop)
			r.rtas.Take(old.attributes)
			return newRoute
		})
	if !added {
		return 0
	}
	return 1
}

// removePrefix removes a route from the RIB. It returns the number of routes really removed.
func (r *rib) removePrefix(ip netip.Addr, bits int, oldRoute route) int {
	v6 := patricia.NewIPv6Address(ip.AsSlice(), uint(bits))
	removed := r.tree.Delete(v6, func(r1, r2 route) bool {
		// This is not enforced/documented, but the route in the tree is the first one.
		if r1.peer == r2.peer && r1.nlri == r2.nlri {
			r.nlris.Take(r1.nlri)
			r.nextHops.Take(r1.nextHop)
			r.rtas.Take(r1.attributes)
			return true
		}
		return false
	}, oldRoute)
	return removed
}

// flushPeer removes a whole peer from the RIB, returning the number
// of removed routes.
func (r *rib) flushPeer(peer uint32) int {
	removed, _ := r.flushPeerContext(nil, peer, 0)
	return removed
}

// flushPeerContext removes a whole peer from the RIB, with a context returning
// the number of removed routes and a bool to say if the operation was completed
// before cancellation.
func (r *rib) flushPeerContext(ctx context.Context, peer uint32, steps int) (int, bool) {
	done := atomic.Bool{}
	stop := make(chan struct{})
	lastStep := 0
	if ctx != nil {
		defer close(stop)
		go func() {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				done.Store(true)
			}
		}()
	}

	// Flush routes
	removed := 0
	buf := make([]route, 0)
	iter := r.tree.Iterate()
	runtime.Gosched()
	for iter.Next() {
		removed += iter.DeleteWithBuffer(buf, func(payload route, _ route) bool {
			if payload.peer == peer {
				r.nlris.Take(payload.nlri)
				r.nextHops.Take(payload.nextHop)
				r.rtas.Take(payload.attributes)
				return true
			}
			return false
		}, route{})
		if ctx != nil && removed/steps > lastStep {
			runtime.Gosched()
			lastStep = removed / steps
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
		nlris:    intern.NewPool[nlri](),
		nextHops: intern.NewPool[nextHop](),
		rtas:     intern.NewPool[routeAttributes](),
	}
}
