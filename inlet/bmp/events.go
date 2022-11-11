// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"time"

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
	reference          uint32                   // used as a reference in the RIB
	staleUntil         time.Time                // when to remove because it is stale
	marshallingOptions []*bgp.MarshallingOption // decoding option (add-path mostly)
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

// scheduleStalePeersRemoval schedule the next time a peer should be
// removed. This should be called with the lock held.
func (c *Component) scheduleStalePeersRemoval() {
	var next time.Time
	for _, pinfo := range c.peers {
		if pinfo.staleUntil.IsZero() {
			continue
		}
		if next.IsZero() || pinfo.staleUntil.Before(next) {
			next = pinfo.staleUntil
		}
	}
	if next.IsZero() {
		c.r.Debug().Msg("no stale peer")
		c.staleTimer.Stop()
	} else {
		c.r.Debug().Msgf("next removal for stale peer scheduled on %s", next)
		c.staleTimer.Reset(c.d.Clock.Until(next))
	}
}

// removeStalePeers remove the stale peers.
func (c *Component) removeStalePeers() {
	start := c.d.Clock.Now()
	c.r.Debug().Msg("remove stale peers")
	c.mu.Lock()
	defer c.mu.Unlock()
	for pkey, pinfo := range c.peers {
		if pinfo.staleUntil.IsZero() || pinfo.staleUntil.After(start) {
			continue
		}
		c.removePeer(pkey, "stale")
	}
	c.scheduleStalePeersRemoval()
}

func (c *Component) addPeer(pkey peerKey) *peerInfo {
	c.lastPeerReference++
	if c.lastPeerReference == 0 {
		// This is a very unlikely event, but we don't
		// have anything better. Let's crash (and
		// hopefully be restarted).
		c.r.Fatal().Msg("too many peer up events")
		go c.Stop()
	}
	pinfo := &peerInfo{
		reference: c.lastPeerReference,
	}
	c.peers[pkey] = pinfo
	return pinfo
}

// removePeer remove a peer (with lock held)
func (c *Component) removePeer(pkey peerKey, reason string) {
	exporterStr := pkey.exporter.Addr().Unmap().String()
	peerStr := pkey.ip.Unmap().String()
	c.r.Info().Msgf("remove peer %s for exporter %s (reason: %s)", peerStr, exporterStr, reason)
	select {
	case c.peerRemovalChan <- pkey:
		return
	default:
	}
	c.metrics.peerRemovalQueueFull.WithLabelValues(exporterStr).Inc()
	c.mu.Unlock()
	select {
	case c.peerRemovalChan <- pkey:
	case <-c.t.Dying():
	}
	c.mu.Lock()
}

// markExporterAsStale marks all peers from an exporter as stale.
func (c *Component) markExporterAsStale(exporter netip.AddrPort, until time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for pkey, pinfo := range c.peers {
		if pkey.exporter != exporter {
			continue
		}
		pinfo.staleUntil = until
	}
	c.scheduleStalePeersRemoval()
}

// handlePeerDownNotification handles a peer-down notification by
// removing the peer.
func (c *Component) handlePeerDownNotification(pkey peerKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.peers[pkey]
	if !ok {
		c.r.Info().Msgf("received peer down from exporter %s for peer %s, but no peer up",
			pkey.exporter.Addr().Unmap().String(),
			pkey.ip.Unmap().String())
		return
	}
	c.removePeer(pkey, "down")
}

// handleConnectionDown handles a disconnect or a session termination
// by marking all associated peers as stale.
func (c *Component) handleConnectionDown(exporter netip.AddrPort) {
	until := c.d.Clock.Now().Add(c.config.Keep)
	c.markExporterAsStale(exporter, until)
}

// handleConnectionUp handles the connection from a new exporter.
func (c *Component) handleConnectionUp(exporter netip.AddrPort) {
	exporterStr := exporter.Addr().Unmap().String()
	// Do not set to 0, exporterStr may cover several exporters.
	c.metrics.peers.WithLabelValues(exporterStr).Add(0)
	c.metrics.routes.WithLabelValues(exporterStr).Add(0)
}

// handlePeerUpNotification handles a new peer.
func (c *Component) handlePeerUpNotification(pkey peerKey, body *bmp.BMPPeerUpNotification) {
	if body.ReceivedOpenMsg == nil || body.SentOpenMsg == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	exporterStr := pkey.exporter.Addr().Unmap().String()
	peerStr := pkey.ip.Unmap().String()
	pinfo, ok := c.peers[pkey]
	if ok {
		c.r.Info().Msgf("received extra peer up from exporter %s for peer %s",
			exporterStr, peerStr)
	} else {
		// Peer does not exist at all
		c.metrics.peers.WithLabelValues(exporterStr).Inc()
		pinfo = c.addPeer(pkey)
	}

	// Check for ADD-PATH support.
	receivedAddPath := map[bgp.RouteFamily]bgp.BGPAddPathMode{}
	received, _ := body.ReceivedOpenMsg.Body.(*bgp.BGPOpen)
	for _, param := range received.OptParams {
		switch param := param.(type) {
		case *bgp.OptionParameterCapability:
			for _, cap := range param.Capability {
				switch cap := cap.(type) {
				case *bgp.CapAddPath:
					for _, tuple := range cap.Tuples {
						receivedAddPath[tuple.RouteFamily] = tuple.Mode
					}
				}
			}
		}
	}
	sent, _ := body.SentOpenMsg.Body.(*bgp.BGPOpen)
	addPathOption := map[bgp.RouteFamily]bgp.BGPAddPathMode{}
	for _, param := range sent.OptParams {
		switch param := param.(type) {
		case *bgp.OptionParameterCapability:
			for _, cap := range param.Capability {
				switch cap := cap.(type) {
				case *bgp.CapAddPath:
					for _, sent := range cap.Tuples {
						receivedMode := receivedAddPath[sent.RouteFamily]
						if receivedMode == bgp.BGP_ADD_PATH_BOTH || receivedMode == bgp.BGP_ADD_PATH_SEND {
							if sent.Mode == bgp.BGP_ADD_PATH_BOTH || sent.Mode == bgp.BGP_ADD_PATH_RECEIVE {
								// We have at least the receive mode. We only do decoding.
								addPathOption[sent.RouteFamily] = bgp.BGP_ADD_PATH_RECEIVE
							}
						}
					}
				}
			}
		}
	}
	pinfo.marshallingOptions = []*bgp.MarshallingOption{{AddPath: addPathOption}}

	c.r.Debug().
		Str("addpath", fmt.Sprintf("%s", addPathOption)).
		Msgf("new peer %s from exporter %s", peerStr, exporterStr)
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

	c.mu.Lock()
	defer c.mu.Unlock()

	// Ignore this peer if this is a L3VPN and it does not have
	// the right RD.
	if pkey.ptype == bmp.BMP_PEER_TYPE_L3VPN && !c.isAcceptedRD(pkey.distinguisher) {
		return
	}

	exporterStr := pkey.exporter.Addr().Unmap().String()
	peerStr := pkey.ip.Unmap().String()
	pinfo, ok := c.peers[pkey]
	if !ok {
		// We may have missed the peer down notification?
		c.r.Info().Msgf("received route monitoring from exporter %s for peer %s, but no peer up",
			exporterStr, peerStr)
		c.metrics.peers.WithLabelValues(exporterStr).Inc()
		pinfo = c.addPeer(pkey)
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
			added += c.rib.addPrefix(p, plen, route{
				peer: pinfo.reference,
				nlri: nlri{
					family: routeFamily(bgp.RF_IPv4_UC),
					path:   ipprefix.PathIdentifier(),
					rd:     pkey.distinguisher,
				},
				nextHop:    c.rib.nextHops.Put(nextHop(nh)),
				attributes: c.rib.rtas.Put(rta),
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
			removed += c.rib.removePrefix(p, plen, route{
				peer: pinfo.reference,
				nlri: nlri{
					family: routeFamily(bgp.RF_IPv4_UC),
					path:   ipprefix.PathIdentifier(),
					rd:     pkey.distinguisher,
				},
			})
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
				added += c.rib.addPrefix(p, plen, route{
					peer: pinfo.reference,
					nlri: nlri{
						family: routeFamily(bgp.AfiSafiToRouteFamily(ipprefix.AFI(), ipprefix.SAFI())),
						rd:     rd,
						path:   ipprefix.PathIdentifier(),
					},
					nextHop:    c.rib.nextHops.Put(nextHop(nh)),
					attributes: c.rib.rtas.Put(rta),
				})
			case *bgp.PathAttributeMpUnreachNLRI:
				removed += c.rib.removePrefix(p, plen, route{
					peer: pinfo.reference,
					nlri: nlri{
						family: routeFamily(bgp.AfiSafiToRouteFamily(ipprefix.AFI(), ipprefix.SAFI())),
						rd:     rd,
						path:   ipprefix.PathIdentifier(),
					},
				})
			}
		}
	}

	c.metrics.routes.WithLabelValues(exporterStr).Add(float64(added - removed))
}

func (c *Component) isAcceptedRD(rd RD) bool {
	if len(c.acceptedRDs) == 0 {
		return true
	}
	_, ok := c.acceptedRDs[uint64(rd)]
	return ok
}
