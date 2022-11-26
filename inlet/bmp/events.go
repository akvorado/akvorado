// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"context"
	"encoding/binary"
	"errors"
	"net/netip"
	"time"

	"akvorado/common/reporter"

	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
	"github.com/osrg/gobgp/v3/pkg/packet/bmp"
)

// peerKey is the key used to identify a peer
type peerKey struct {
	exporter      netip.AddrPort // exporter IP + source port
	ip            netip.Addr     // peer IP
	ptype         uint8          // peer type
	distinguisher RD             // peer distinguisher
	asn           uint32         // peer ASN
	bgpID         uint32         // peer router ID
}

// peerInfo contains some information attached to a peer.
type peerInfo struct {
	reference  uint32    // used as a reference in the RIB
	staleUntil time.Time // when to remove because it is stale
}

// peerKeyFromBMPPeerHeader computes the peer key from the BMP peer header.
func peerKeyFromBMPPeerHeader(exporter netip.AddrPort, header *bmp.BMPPeerHeader) peerKey {
	peer, _ := netip.AddrFromSlice(header.PeerAddress.To16())
	return peerKey{
		exporter:      exporter,
		ip:            peer,
		ptype:         header.PeerType,
		distinguisher: RD(header.PeerDistinguisher),
		asn:           header.PeerAS,
		bgpID:         binary.BigEndian.Uint32(header.PeerBGPID.To4()),
	}
}

// ribWorkerState is the state of the rib worker (accessible through the worker
// only).
type ribWorkerState struct {
	ctx               context.Context
	rib               *rib
	peers             map[peerKey]*peerInfo
	peerLastReference uint32
}

// ribWorkerPayload is what we provide the RIB worker with. The channel will be
// closed when done.
type ribWorkerPayload struct {
	fn            func(*ribWorkerState) error
	interruptible bool
	done          chan<- struct{}
}

var errRIBWorkerCanceled = errors.New("RIB worker timeout")

// ribWorker handle RIB-related operations (everything involving updating RIB
// and related structures). Tasks are functions queued inside the worker.
func (c *Component) ribWorker() error {
	state := &ribWorkerState{
		rib:   newRIB(),
		peers: make(map[peerKey]*peerInfo),
	}
	// Assume the last copy was done before minimum update delay
	lastCopy := time.Now().Add(-c.config.RIBMinimumUpdateDelay)
	nextTimer := time.NewTimer(c.config.RIBMaximumUpdateDelay)
	timer := "maximum"
	uptodate := true
	priorityPayloads := make(chan ribWorkerPayload, 1)
	pausedPayloads := make(chan ribWorkerPayload, 1)

	if c.config.RIBMode == RIBModePerformance {
		c.r.GaugeFunc(
			reporter.GaugeOpts{
				Name: "rib_lag_seconds",
				Help: "How outdated is the readonly RIB.",
			},
			func() float64 {
				if uptodate {
					return 0
				}
				return time.Now().Sub(lastCopy).Seconds()
			},
		)
	}

	handleLowPriorityPayload := func(payload ribWorkerPayload) error {
		// These low priority operations can be canceled when a high priority request happens.
		if c.config.RIBMode == RIBModeMemory && payload.interruptible {
			ctx, cancel := context.WithCancel(c.t.Context(context.Background()))
			state.ctx = ctx
			defer cancel()
			go func() {
				select {
				case <-c.t.Dying():
				case <-ctx.Done():
				case payload := <-c.ribWorkerPrioChan:
					priorityPayloads <- payload
					cancel()
				}
			}()
		}

		uptodate = false
		err := payload.fn(state)
		if err == errRIBWorkerCanceled {
			pausedPayloads <- payload
			return nil
		}
		close(payload.done)
		if err != nil {
			return err
		}

		if c.config.RIBMode == RIBModePerformance {
			if !nextTimer.Stop() {
				select {
				case <-nextTimer.C:
				default:
				}
			}
			now := time.Now()
			delta := now.Sub(lastCopy)
			if delta < c.config.RIBMinimumUpdateDelay {
				nextTimer.Reset(c.config.RIBMinimumUpdateDelay - delta)
				timer = "minimum"
			} else if delta < c.config.RIBMaximumUpdateDelay-c.config.RIBIdleUpdateDelay {
				nextTimer.Reset(c.config.RIBIdleUpdateDelay)
				timer = "idle"
			} else if delta >= c.config.RIBMaximumUpdateDelay {
				c.updateRIBReadonly(state, "maximum")
				lastCopy = now
				uptodate = true
			} else {
				nextTimer.Reset(c.config.RIBMaximumUpdateDelay - delta)
				timer = "maximum"
			}
		}
		return nil
	}

	for {
		state.ctx = nil
		select {
		// Two "high priority" chans: one for internal, one for external
		case payload := <-priorityPayloads:
			err := payload.fn(state)
			close(payload.done)
			if err != nil {
				return err
			}
		case payload := <-c.ribWorkerPrioChan:
			err := payload.fn(state)
			close(payload.done)
			if err != nil {
				return err
			}
		default:
			select {
			case <-c.t.Dying():
				return nil
			// No need to watch for internal high priority, it should have been
			// handled before. We can still get high priority requests from the
			// external one.
			case payload := <-c.ribWorkerPrioChan:
				err := payload.fn(state)
				close(payload.done)
				if err != nil {
					return err
				}
			case payload := <-pausedPayloads:
				if err := handleLowPriorityPayload(payload); err != nil {
					return err
				}
			case payload := <-c.ribWorkerChan:
				if err := handleLowPriorityPayload(payload); err != nil {
					return err
				}
			case <-nextTimer.C:
				c.updateRIBReadonly(state, timer)
				lastCopy = time.Now()
				uptodate = true
			}
		}
	}
}

type ribWorkerQueueOptions int

const (
	ribWorkerHighPriority ribWorkerQueueOptions = iota
	ribWorkerInterruptible
)

// ribWorkerQueue queues a new task for the RIB worker thread. We wait for the
// task to be handled. We don't want to queue up a lot of tasks asynchronously.
func (c *Component) ribWorkerQueue(fn func(*ribWorkerState) error, options ...ribWorkerQueueOptions) {
	ch := c.ribWorkerChan
	done := make(chan struct{})
	payload := ribWorkerPayload{
		fn:   fn,
		done: done,
	}
	for _, option := range options {
		switch option {
		case ribWorkerHighPriority:
			ch = c.ribWorkerPrioChan
		case ribWorkerInterruptible:
			payload.interruptible = true
		}
	}
	select {
	case <-c.t.Dying():
	case ch <- payload:
		select {
		case <-c.t.Dying():
		case <-done:
		}
	}
}

// updateRIBReadonly updates the read-only copy of the RIB
func (c *Component) updateRIBReadonly(s *ribWorkerState, timer string) {
	if c.config.RIBMode == RIBModePerformance {
		c.r.Debug().Msg("copy live RIB to read-only version")
		start := time.Now()
		defer c.metrics.ribCopies.WithLabelValues(timer).Observe(
			float64(time.Now().Sub(start).Nanoseconds()) / 1000 / 1000 / 1000)
		new := s.rib.clone()
		c.ribReadonly.Store(new)
	}
}

// addPeer provides a reference to a new peer.
func (c *Component) addPeer(s *ribWorkerState, pkey peerKey) *peerInfo {
	s.peerLastReference++
	if s.peerLastReference == 0 {
		// This is a very unlikely event, but we don't
		// have anything better. Let's crash (and
		// hopefully be restarted).
		c.r.Fatal().Msg("too many peer up events")
		go c.Stop()
	}
	pinfo := &peerInfo{
		reference: s.peerLastReference,
	}
	s.peers[pkey] = pinfo
	return pinfo
}

// scheduleStalePeersRemoval schedule the next time a peer should be removed.
func (c *Component) scheduleStalePeersRemoval(s *ribWorkerState) {
	var next time.Time
	for _, pinfo := range s.peers {
		if pinfo.staleUntil.IsZero() {
			continue
		}
		if next.IsZero() || pinfo.staleUntil.Before(next) {
			next = pinfo.staleUntil
		}
	}
	if next.IsZero() {
		c.r.Debug().Msg("no stale peer")
		c.peerStaleTimer.Stop()
	} else {
		c.r.Debug().Msgf("next removal for stale peer scheduled on %s", next)
		c.peerStaleTimer.Reset(c.d.Clock.Until(next))
	}
}

// removePeer remove a peer.
func (c *Component) removePeer(s *ribWorkerState, pkey peerKey, reason string) error {
	exporterStr := pkey.exporter.Addr().Unmap().String()
	peerStr := pkey.ip.Unmap().String()
	pinfo := s.peers[pkey]
	c.r.Info().Msgf("remove peer %s for exporter %s (reason: %s)", peerStr, exporterStr, reason)
	removed, done := s.rib.flushPeerContext(s.ctx, pinfo.reference, c.config.RIBPeerRemovalBatchRoutes)
	c.metrics.routes.WithLabelValues(exporterStr).Sub(float64(removed))
	if !done {
		c.metrics.peerRemovalPartial.WithLabelValues(exporterStr).Inc()
		return errRIBWorkerCanceled
	}
	c.metrics.peerRemovalDone.WithLabelValues(exporterStr).Inc()
	c.metrics.peers.WithLabelValues(exporterStr).Dec()
	delete(s.peers, pkey)
	return nil
}

// handleStalePeers remove the stale peers.
func (c *Component) handleStalePeers() {
	c.ribWorkerQueue(func(s *ribWorkerState) error {
		start := c.d.Clock.Now()
		c.r.Debug().Msg("remove stale peers")
		for pkey, pinfo := range s.peers {
			if pinfo.staleUntil.IsZero() || pinfo.staleUntil.After(start) {
				continue
			}
			if err := c.removePeer(s, pkey, "stale"); err != nil {
				return err
			}
		}
		c.scheduleStalePeersRemoval(s)
		return nil
	}, ribWorkerInterruptible)
}

// handlePeerDownNotification handles a peer-down notification by
// removing the peer.
func (c *Component) handlePeerDownNotification(pkey peerKey) {
	c.ribWorkerQueue(func(s *ribWorkerState) error {
		_, ok := s.peers[pkey]
		if !ok {
			c.r.Info().Msgf("received peer down from exporter %s for peer %s, but no peer up",
				pkey.exporter.Addr().Unmap().String(),
				pkey.ip.Unmap().String())
			return nil
		}
		return c.removePeer(s, pkey, "down")
	}, ribWorkerInterruptible)
}

// handleConnectionDown handles a disconnect or a session termination
// by marking all associated peers as stale.
func (c *Component) handleConnectionDown(exporter netip.AddrPort) {
	until := c.d.Clock.Now().Add(c.config.Keep)
	c.ribWorkerQueue(func(s *ribWorkerState) error {
		for pkey, pinfo := range s.peers {
			if pkey.exporter != exporter {
				continue
			}
			pinfo.staleUntil = until
		}
		c.scheduleStalePeersRemoval(s)
		return nil
	})
}

// handleConnectionUp handles the connection from a new exporter.
func (c *Component) handleConnectionUp(exporter netip.AddrPort) {
	// Do it without RIB worker, we just update counters.
	// Do not set to 0, exporterStr may cover several exporters.
	exporterStr := exporter.Addr().Unmap().String()
	c.metrics.peers.WithLabelValues(exporterStr).Add(0)
	c.metrics.routes.WithLabelValues(exporterStr).Add(0)
}

// handlePeerUpNotification handles a new peer.
func (c *Component) handlePeerUpNotification(pkey peerKey, body *bmp.BMPPeerUpNotification) {
	if body.ReceivedOpenMsg == nil || body.SentOpenMsg == nil {
		return
	}

	c.ribWorkerQueue(func(s *ribWorkerState) error {
		exporterStr := pkey.exporter.Addr().Unmap().String()
		peerStr := pkey.ip.Unmap().String()
		_, ok := s.peers[pkey]
		if ok {
			c.r.Info().Msgf("received extra peer up from exporter %s for peer %s",
				exporterStr, peerStr)
		} else {
			// Peer does not exist at all
			c.metrics.peers.WithLabelValues(exporterStr).Inc()
			c.addPeer(s, pkey)
		}

		c.r.Debug().Msgf("new peer %s from exporter %s", peerStr, exporterStr)
		return nil
	})
}

func (c *Component) handleRouteMonitoring(pkey peerKey, body *bmp.BMPRouteMonitoring) {
	// We expect to have a BGP update message
	if body.BGPUpdate == nil || body.BGPUpdate.Body == nil {
		return
	}
	update, ok := body.BGPUpdate.Body.(*bgp.BGPUpdate)
	if !ok {
		return
	}

	c.ribWorkerQueue(func(s *ribWorkerState) error {

		// Ignore this peer if this is a L3VPN and it does not have
		// the right RD.
		if pkey.ptype == bmp.BMP_PEER_TYPE_L3VPN && !c.isAcceptedRD(pkey.distinguisher) {
			return nil
		}

		exporterStr := pkey.exporter.Addr().Unmap().String()
		peerStr := pkey.ip.Unmap().String()
		pinfo, ok := s.peers[pkey]
		if !ok {
			// We may have missed the peer down notification?
			c.r.Info().Msgf("received route monitoring from exporter %s for peer %s, but no peer up",
				exporterStr, peerStr)
			c.metrics.peers.WithLabelValues(exporterStr).Inc()
			pinfo = c.addPeer(s, pkey)
		}

		var nh netip.Addr
		var rta routeAttributes
		for _, attr := range update.PathAttributes {
			switch attr := attr.(type) {
			case *bgp.PathAttributeNextHop:
				nh, _ = netip.AddrFromSlice(attr.Value.To16())
			case *bgp.PathAttributeAsPath:
				if c.config.CollectASNs || c.config.CollectASPaths {
					rta.asPath = asPathFlat(attr)
				}
			case *bgp.PathAttributeCommunities:
				if c.config.CollectCommunities {
					rta.communities = attr.Value
				}
			case *bgp.PathAttributeLargeCommunities:
				if c.config.CollectCommunities {
					rta.largeCommunities = make([]bgp.LargeCommunity, len(attr.Values))
					for idx, c := range attr.Values {
						rta.largeCommunities[idx] = *c
					}
				}
			}
		}
		// If no AS path, consider the peer AS as the origin AS,
		// otherwise the last AS.
		if c.config.CollectASNs {
			if path := rta.asPath; len(path) == 0 {
				rta.asn = pkey.asn
			} else {
				rta.asn = path[len(path)-1]
			}
		}
		if !c.config.CollectASPaths {
			rta.asPath = rta.asPath[:0]
		}

		added := 0
		removed := 0

		// Regular NLRI and withdrawn routes
		if pkey.ptype == bmp.BMP_PEER_TYPE_L3VPN || c.isAcceptedRD(0) {
			for _, ipprefix := range update.NLRI {
				prefix := ipprefix.Prefix
				plen := int(ipprefix.Length)
				if prefix.To4() != nil {
					prefix = prefix.To16()
					plen += 96
				}
				p, _ := netip.AddrFromSlice(prefix)
				added += s.rib.addPrefix(p, plen, route{
					peer: pinfo.reference,
					nlri: s.rib.nlris.Put(nlri{
						family: bgp.RF_IPv4_UC,
						path:   ipprefix.PathIdentifier(),
						rd:     pkey.distinguisher,
					}),
					nextHop:    s.rib.nextHops.Put(nextHop(nh)),
					attributes: s.rib.rtas.Put(rta),
				})
			}
			for _, ipprefix := range update.WithdrawnRoutes {
				prefix := ipprefix.Prefix
				plen := int(ipprefix.Length)
				if prefix.To4() != nil {
					prefix = prefix.To16()
					plen += 96
				}
				p, _ := netip.AddrFromSlice(prefix)
				if nlriRef, ok := s.rib.nlris.Ref(nlri{
					family: bgp.RF_IPv4_UC,
					path:   ipprefix.PathIdentifier(),
					rd:     pkey.distinguisher,
				}); ok {
					removed += s.rib.removePrefix(p, plen, route{
						peer: pinfo.reference,
						nlri: nlriRef,
					})
				}
			}
		}

		// MP reach and unreach NLRI
		for _, attr := range update.PathAttributes {
			var p netip.Addr
			var plen int
			var rd RD
			var ipprefixes []bgp.AddrPrefixInterface
			switch attr := attr.(type) {
			case *bgp.PathAttributeMpReachNLRI:
				nh, _ = netip.AddrFromSlice(attr.Nexthop.To16())
				ipprefixes = attr.Value
			case *bgp.PathAttributeMpUnreachNLRI:
				ipprefixes = attr.Value
			}
			for _, ipprefix := range ipprefixes {
				switch ipprefix := ipprefix.(type) {
				case *bgp.IPAddrPrefix:
					p, _ = netip.AddrFromSlice(ipprefix.Prefix.To16())
					plen = int(ipprefix.Length + 96)
					rd = pkey.distinguisher
				case *bgp.IPv6AddrPrefix:
					p, _ = netip.AddrFromSlice(ipprefix.Prefix.To16())
					plen = int(ipprefix.Length)
					rd = pkey.distinguisher
				case *bgp.LabeledIPAddrPrefix:
					p, _ = netip.AddrFromSlice(ipprefix.Prefix.To16())
					plen = int(ipprefix.IPPrefixLen() + 96)
					rd = pkey.distinguisher
				case *bgp.LabeledIPv6AddrPrefix:
					p, _ = netip.AddrFromSlice(ipprefix.Prefix.To16())
					plen = int(ipprefix.IPPrefixLen())
					rd = pkey.distinguisher
				case *bgp.LabeledVPNIPAddrPrefix:
					p, _ = netip.AddrFromSlice(ipprefix.Prefix.To16())
					plen = int(ipprefix.IPPrefixLen() + 96)
					rd = RDFromRouteDistinguisherInterface(ipprefix.RD)
				case *bgp.LabeledVPNIPv6AddrPrefix:
					p, _ = netip.AddrFromSlice(ipprefix.Prefix.To16())
					plen = int(ipprefix.IPPrefixLen())
					rd = RDFromRouteDistinguisherInterface(ipprefix.RD)
				case *bgp.EVPNNLRI:
					switch route := ipprefix.RouteTypeData.(type) {
					case *bgp.EVPNIPPrefixRoute:
						prefix := route.IPPrefix
						plen = int(route.IPPrefixLength)
						if prefix.To4() != nil {
							prefix = prefix.To16()
							plen += 96
						}
						p, _ = netip.AddrFromSlice(prefix.To16())
						rd = RDFromRouteDistinguisherInterface(route.RD)
					}
				default:
					c.metrics.ignoredNlri.WithLabelValues(exporterStr,
						bgp.AfiSafiToRouteFamily(ipprefix.AFI(), ipprefix.SAFI()).String()).Inc()
					continue
				}
				if pkey.ptype != bmp.BMP_PEER_TYPE_L3VPN && !c.isAcceptedRD(rd) {
					continue
				}
				switch attr.(type) {
				case *bgp.PathAttributeMpReachNLRI:
					added += s.rib.addPrefix(p, plen, route{
						peer: pinfo.reference,
						nlri: s.rib.nlris.Put(nlri{
							family: bgp.AfiSafiToRouteFamily(ipprefix.AFI(), ipprefix.SAFI()),
							rd:     rd,
							path:   ipprefix.PathIdentifier(),
						}),
						nextHop:    s.rib.nextHops.Put(nextHop(nh)),
						attributes: s.rib.rtas.Put(rta),
					})
				case *bgp.PathAttributeMpUnreachNLRI:
					if nlriRef, ok := s.rib.nlris.Ref(nlri{
						family: bgp.AfiSafiToRouteFamily(ipprefix.AFI(), ipprefix.SAFI()),
						rd:     rd,
						path:   ipprefix.PathIdentifier(),
					}); ok {
						removed += s.rib.removePrefix(p, plen, route{
							peer: pinfo.reference,
							nlri: nlriRef,
						})
					}
				}
			}
		}

		c.metrics.routes.WithLabelValues(exporterStr).Add(float64(added - removed))
		return nil
	})
}

func (c *Component) isAcceptedRD(rd RD) bool {
	if len(c.acceptedRDs) == 0 {
		return true
	}
	_, ok := c.acceptedRDs[uint64(rd)]
	return ok
}
