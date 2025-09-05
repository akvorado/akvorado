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
func (p *Provider) scheduleStalePeersRemoval() {
	var next time.Time
	for _, pinfo := range p.peers {
		if pinfo.staleUntil.IsZero() {
			continue
		}
		if next.IsZero() || pinfo.staleUntil.Before(next) {
			next = pinfo.staleUntil
		}
	}
	if next.IsZero() {
		p.r.Debug().Msg("no stale peer")
		p.staleTimer.Stop()
	} else {
		p.r.Debug().Msgf("next removal for stale peer scheduled on %s", next)
		p.staleTimer.Reset(p.d.Clock.Until(next))
	}
}

// removeStalePeers remove the stale peers.
func (p *Provider) removeStalePeers() {
	start := p.d.Clock.Now()
	p.r.Debug().Msg("remove stale peers")
	p.mu.Lock()
	defer p.mu.Unlock()
	for pkey, pinfo := range p.peers {
		if pinfo.staleUntil.IsZero() || pinfo.staleUntil.After(start) {
			continue
		}
		p.removePeer(pkey, "stale")
	}
	p.scheduleStalePeersRemoval()
}

func (p *Provider) addPeer(pkey peerKey) *peerInfo {
	p.lastPeerReference++
	if p.lastPeerReference == 0 {
		// This is a very unlikely event, but we don't
		// have anything better. Let's crash (and
		// hopefully be restarted).
		p.r.Fatal().Msg("too many peer up events")
		go p.Stop()
	}
	pinfo := &peerInfo{
		reference: p.lastPeerReference,
	}
	p.peers[pkey] = pinfo
	return pinfo
}

// removePeer remove a peer (with lock held)
func (p *Provider) removePeer(pkey peerKey, reason string) {
	exporterStr := pkey.exporter.Addr().Unmap().String()
	peerStr := pkey.ip.Unmap().String()
	p.r.Info().Msgf("remove peer %s for exporter %s (reason: %s)", peerStr, exporterStr, reason)
	start := p.d.Clock.Now()
	defer p.metrics.locked.WithLabelValues("peer-removal").Observe(
		float64(p.d.Clock.Now().Sub(start).Nanoseconds()) / 1000 / 1000 / 1000)
	pinfo, ok := p.peers[pkey]
	if !ok {
		return
	}
	removed := p.rib.FlushPeer(pinfo.reference)
	delete(p.peers, pkey)
	p.metrics.routes.WithLabelValues(exporterStr).Sub(float64(removed))
	p.metrics.peers.WithLabelValues(exporterStr).Dec()
	p.metrics.peerRemovalDone.WithLabelValues(exporterStr).Inc()
}

// markExporterAsStale marks all peers from an exporter as stale.
func (p *Provider) markExporterAsStale(exporter netip.AddrPort, until time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for pkey, pinfo := range p.peers {
		if pkey.exporter != exporter {
			continue
		}
		pinfo.staleUntil = until
	}
	p.scheduleStalePeersRemoval()
}

// handlePeerDownNotification handles a peer-down notification by
// removing the peer.
func (p *Provider) handlePeerDownNotification(pkey peerKey) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.peers[pkey]
	if !ok {
		p.r.Info().Msgf("received peer down from exporter %s for peer %s, but no peer up",
			pkey.exporter.Addr().Unmap().String(),
			pkey.ip.Unmap().String())
		return
	}
	p.removePeer(pkey, "down")
}

// handleConnectionDown handles a disconnect or a session termination
// by marking all associated peers as stale.
func (p *Provider) handleConnectionDown(exporter netip.AddrPort) {
	until := p.d.Clock.Now().Add(p.config.Keep)
	p.markExporterAsStale(exporter, until)
}

// handleConnectionUp handles the connection from a new exporter.
func (p *Provider) handleConnectionUp(exporter netip.AddrPort) {
	exporterStr := exporter.Addr().Unmap().String()
	// Do not set to 0, exporterStr may cover several exporters.
	p.metrics.peers.WithLabelValues(exporterStr).Add(0)
	p.metrics.routes.WithLabelValues(exporterStr).Add(0)
}

// handlePeerUpNotification handles a new peer.
func (p *Provider) handlePeerUpNotification(pkey peerKey, body *bmp.BMPPeerUpNotification) {
	if body.ReceivedOpenMsg == nil || body.SentOpenMsg == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	exporterStr := pkey.exporter.Addr().Unmap().String()
	peerStr := pkey.ip.Unmap().String()
	pinfo, ok := p.peers[pkey]
	if ok {
		p.r.Info().Msgf("received extra peer up from exporter %s for peer %s",
			exporterStr, peerStr)
	} else {
		// Peer does not exist at all
		p.metrics.peers.WithLabelValues(exporterStr).Inc()
		pinfo = p.addPeer(pkey)
	}

	// Check for ADD-PATH support.
	receivedAddPath := map[bgp.RouteFamily]bgp.BGPAddPathMode{}
	received, _ := body.ReceivedOpenMsg.Body.(*bgp.BGPOpen)
	for _, param := range received.OptParams {
		switch param := param.(type) {
		case *bgp.OptionParameterCapability:
			for _, capability := range param.Capability {
				switch capability := capability.(type) {
				case *bgp.CapAddPath:
					for _, tuple := range capability.Tuples {
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
			for _, capability := range param.Capability {
				switch capability := capability.(type) {
				case *bgp.CapAddPath:
					for _, sent := range capability.Tuples {
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

	p.r.Debug().
		Str("addpath", fmt.Sprintf("%s", addPathOption)).
		Msgf("new peer %s from exporter %s", peerStr, exporterStr)
}

func (p *Provider) handleRouteMonitoring(pkey peerKey, body *bmp.BMPRouteMonitoring) {
	// We expect to have a BGP update message
	if body.BGPUpdate == nil || body.BGPUpdate.Body == nil {
		return
	}
	update, ok := body.BGPUpdate.Body.(*bgp.BGPUpdate)
	if !ok {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Ignore this peer if this is a L3VPN and it does not have
	// the right RD.
	if pkey.ptype == bmp.BMP_PEER_TYPE_L3VPN && !p.isAcceptedRD(pkey.distinguisher) {
		return
	}

	exporterStr := pkey.exporter.Addr().Unmap().String()
	peerStr := pkey.ip.Unmap().String()
	pinfo, ok := p.peers[pkey]
	if !ok {
		// We may have missed the peer down notification?
		p.r.Info().Msgf("received route monitoring from exporter %s for peer %s, but no peer up",
			exporterStr, peerStr)
		p.metrics.peers.WithLabelValues(exporterStr).Inc()
		pinfo = p.addPeer(pkey)
	}

	var nh netip.Addr
	var rta routeAttributes
	for _, attr := range update.PathAttributes {
		switch attr := attr.(type) {
		case *bgp.PathAttributeNextHop:
			nh, _ = netip.AddrFromSlice(attr.Value.To16())
		case *bgp.PathAttributeAsPath:
			if p.config.CollectASNs || p.config.CollectASPaths {
				rta.asPath = asPathFlat(attr)
			}
		case *bgp.PathAttributeCommunities:
			if p.config.CollectCommunities {
				rta.communities = attr.Value
			}
		case *bgp.PathAttributeLargeCommunities:
			if p.config.CollectCommunities {
				rta.largeCommunities = make([]bgp.LargeCommunity, len(attr.Values))
				for idx, c := range attr.Values {
					rta.largeCommunities[idx] = *c
				}
			}
		}
	}
	// If no AS path, consider the peer AS as the origin AS,
	// otherwise the last AS.
	if p.config.CollectASNs {
		if path := rta.asPath; len(path) == 0 {
			rta.asn = pkey.asn
		} else {
			rta.asn = path[len(path)-1]
		}
	}
	if !p.config.CollectASPaths {
		rta.asPath = rta.asPath[:0]
	}

	added := 0
	removed := 0

	// Regular NLRI and withdrawn routes
	if pkey.ptype == bmp.BMP_PEER_TYPE_L3VPN || p.isAcceptedRD(0) {
		for _, ipprefix := range update.NLRI {
			prefix := ipprefix.Prefix
			plen := int(ipprefix.Length)
			if prefix.To4() != nil {
				prefix = prefix.To16()
				plen += 96
			}
			pf, _ := netip.AddrFromSlice(prefix)
			added += p.rib.AddPrefix(netip.PrefixFrom(pf, plen), route{
				peer: pinfo.reference,
				nlri: p.rib.nlris.Put(nlri{
					family: bgp.RF_IPv4_UC,
					path:   ipprefix.PathIdentifier(),
					rd:     pkey.distinguisher,
				}),
				nextHop:    p.rib.nextHops.Put(nextHop(nh)),
				attributes: p.rib.rtas.Put(rta),
				prefixLen:  uint8(plen),
			})
		}
		for _, ipprefix := range update.WithdrawnRoutes {
			prefix := ipprefix.Prefix
			plen := int(ipprefix.Length)
			if prefix.To4() != nil {
				prefix = prefix.To16()
				plen += 96
			}
			pf, _ := netip.AddrFromSlice(prefix)
			if nlriRef, ok := p.rib.nlris.Ref(nlri{
				family: bgp.RF_IPv4_UC,
				path:   ipprefix.PathIdentifier(),
				rd:     pkey.distinguisher,
			}); ok {
				removed += p.rib.RemovePrefix(netip.PrefixFrom(pf, plen), route{
					peer: pinfo.reference,
					nlri: nlriRef,
				})
			}
		}
	}

	// MP reach and unreach NLRI
	for _, attr := range update.PathAttributes {
		var pf netip.Addr
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
				pf, _ = netip.AddrFromSlice(ipprefix.Prefix.To16())
				plen = int(ipprefix.Length + 96)
				rd = pkey.distinguisher
			case *bgp.IPv6AddrPrefix:
				pf, _ = netip.AddrFromSlice(ipprefix.Prefix.To16())
				plen = int(ipprefix.Length)
				rd = pkey.distinguisher
			case *bgp.LabeledIPAddrPrefix:
				pf, _ = netip.AddrFromSlice(ipprefix.Prefix.To16())
				plen = int(ipprefix.IPPrefixLen() + 96)
				rd = pkey.distinguisher
			case *bgp.LabeledIPv6AddrPrefix:
				pf, _ = netip.AddrFromSlice(ipprefix.Prefix.To16())
				plen = int(ipprefix.IPPrefixLen())
				rd = pkey.distinguisher
			case *bgp.LabeledVPNIPAddrPrefix:
				pf, _ = netip.AddrFromSlice(ipprefix.Prefix.To16())
				plen = int(ipprefix.IPPrefixLen() + 96)
				rd = RDFromRouteDistinguisherInterface(ipprefix.RD)
			case *bgp.LabeledVPNIPv6AddrPrefix:
				pf, _ = netip.AddrFromSlice(ipprefix.Prefix.To16())
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
					pf, _ = netip.AddrFromSlice(prefix.To16())
					rd = RDFromRouteDistinguisherInterface(route.RD)
				}
			default:
				p.metrics.ignoredNlri.WithLabelValues(exporterStr,
					bgp.AfiSafiToRouteFamily(ipprefix.AFI(), ipprefix.SAFI()).String()).Inc()
				continue
			}
			if pkey.ptype != bmp.BMP_PEER_TYPE_L3VPN && !p.isAcceptedRD(rd) {
				continue
			}
			switch attr.(type) {
			case *bgp.PathAttributeMpReachNLRI:
				added += p.rib.AddPrefix(netip.PrefixFrom(pf, plen), route{
					peer: pinfo.reference,
					nlri: p.rib.nlris.Put(nlri{
						family: bgp.AfiSafiToRouteFamily(ipprefix.AFI(), ipprefix.SAFI()),
						rd:     rd,
						path:   ipprefix.PathIdentifier(),
					}),
					nextHop:    p.rib.nextHops.Put(nextHop(nh)),
					attributes: p.rib.rtas.Put(rta),
					prefixLen:  uint8(plen),
				})
			case *bgp.PathAttributeMpUnreachNLRI:
				if nlriRef, ok := p.rib.nlris.Ref(nlri{
					family: bgp.AfiSafiToRouteFamily(ipprefix.AFI(), ipprefix.SAFI()),
					rd:     rd,
					path:   ipprefix.PathIdentifier(),
				}); ok {
					removed += p.rib.RemovePrefix(netip.PrefixFrom(pf, plen), route{
						peer: pinfo.reference,
						nlri: nlriRef,
					})
				}
			}
		}
	}

	p.metrics.routes.WithLabelValues(exporterStr).Add(float64(added - removed))
}

func (p *Provider) isAcceptedRD(rd RD) bool {
	if len(p.acceptedRDs) == 0 {
		return true
	}
	_, ok := p.acceptedRDs[rd]
	return ok
}
