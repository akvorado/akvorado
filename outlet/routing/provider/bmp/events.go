// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"time"
	"unique"

	"akvorado/common/helpers"

	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"github.com/osrg/gobgp/v4/pkg/packet/bmp"
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
	return peerKey{
		exporter:      exporter,
		ip:            header.PeerAddress,
		ptype:         header.PeerType,
		distinguisher: RD(header.PeerDistinguisher),
		asn:           header.PeerAS,
		bgpID:         binary.BigEndian.Uint32(header.PeerBGPID.AsSlice()),
	}
}

// scheduleStalePeersRemoval schedule the next time a peer should be
// removed. This should be called with the writer lock held.
func (p *Provider) scheduleStalePeersRemoval() {
	var next time.Time
	p.peers.Range(func(_ peerKey, pinfo peerInfo) bool {
		if pinfo.staleUntil.IsZero() {
			return true
		}
		if next.IsZero() || pinfo.staleUntil.Before(next) {
			next = pinfo.staleUntil
		}
		return true
	})
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
	p.peers.Range(func(pkey peerKey, pinfo peerInfo) bool {
		if pinfo.staleUntil.IsZero() || pinfo.staleUntil.After(start) {
			return true
		}
		p.removePeer(pkey, pinfo, "stale")
		return true
	})
	p.scheduleStalePeersRemoval()
}

func (p *Provider) addPeer(pkey peerKey) peerInfo {
	ref := p.lastPeerReference.Add(1)
	if ref == 0 {
		// This is a very unlikely event, but we don't
		// have anything better. Let's crash (and
		// hopefully be restarted).
		p.r.Fatal().Msg("too many peer up events")
		go p.Stop()
	}
	pinfo := peerInfo{
		reference: ref,
	}
	p.peers.Store(pkey, pinfo)
	return pinfo
}

// removePeer remove a peer (with writer lock held).
func (p *Provider) removePeer(pkey peerKey, pinfo peerInfo, reason string) {
	exporterStr := pkey.exporter.Addr().Unmap().String()
	peerStr := pkey.ip.Unmap().String()
	p.r.Info().Msgf("remove peer %s for exporter %s (reason: %s)", peerStr, exporterStr, reason)
	removed := p.rib.FlushPeer(pinfo.reference)
	p.peers.Delete(pkey)
	p.metrics.routes.WithLabelValues(exporterStr).Sub(float64(removed))
	p.metrics.peers.WithLabelValues(exporterStr).Dec()
	p.metrics.peerRemovalDone.WithLabelValues(exporterStr).Inc()
}

// markExporterAsStale marks all peers from an exporter as stale.
func (p *Provider) markExporterAsStale(exporter netip.AddrPort, until time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers.Range(func(pkey peerKey, pinfo peerInfo) bool {
		if pkey.exporter != exporter {
			return true
		}
		newPinfo := peerInfo{
			reference:          pinfo.reference,
			staleUntil:         until,
			marshallingOptions: pinfo.marshallingOptions,
		}
		p.peers.Store(pkey, newPinfo)
		return true
	})
	p.scheduleStalePeersRemoval()
}

// handlePeerDownNotification handles a peer-down notification by
// removing the peer.
func (p *Provider) handlePeerDownNotification(pkey peerKey) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pinfo, ok := p.peers.Load(pkey)
	if !ok {
		p.r.Info().Msgf("received peer down from exporter %s for peer %s, but no peer up",
			pkey.exporter.Addr().Unmap().String(),
			pkey.ip.Unmap().String())
		return
	}
	p.removePeer(pkey, pinfo, "down")
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
	var pinfo peerInfo
	if oldPinfo, ok := p.peers.Load(pkey); ok {
		p.r.Info().Msgf("received extra peer up from exporter %s for peer %s",
			exporterStr, peerStr)
		// Create new peerInfo preserving the reference (old one stays valid for concurrent readers)
		pinfo = peerInfo{
			reference:  oldPinfo.reference,
			staleUntil: oldPinfo.staleUntil,
		}
	} else {
		// Peer does not exist at all
		p.metrics.peers.WithLabelValues(exporterStr).Inc()
		pinfo = p.addPeer(pkey)
	}

	// Check for ADD-PATH support.
	receivedAddPath := map[bgp.Family]bgp.BGPAddPathMode{}
	received, _ := body.ReceivedOpenMsg.Body.(*bgp.BGPOpen)
	for _, param := range received.OptParams {
		switch param := param.(type) {
		case *bgp.OptionParameterCapability:
			for _, capability := range param.Capability {
				switch capability := capability.(type) {
				case *bgp.CapAddPath:
					for _, tuple := range capability.Tuples {
						receivedAddPath[tuple.Family] = tuple.Mode
					}
				}
			}
		}
	}
	sent, _ := body.SentOpenMsg.Body.(*bgp.BGPOpen)
	addPathOption := map[bgp.Family]bgp.BGPAddPathMode{}
	for _, param := range sent.OptParams {
		switch param := param.(type) {
		case *bgp.OptionParameterCapability:
			for _, capability := range param.Capability {
				switch capability := capability.(type) {
				case *bgp.CapAddPath:
					for _, sent := range capability.Tuples {
						receivedMode := receivedAddPath[sent.Family]
						if receivedMode == bgp.BGP_ADD_PATH_BOTH || receivedMode == bgp.BGP_ADD_PATH_SEND {
							if sent.Mode == bgp.BGP_ADD_PATH_BOTH || sent.Mode == bgp.BGP_ADD_PATH_RECEIVE {
								// We have at least the receive mode. We only do decoding.
								addPathOption[sent.Family] = bgp.BGP_ADD_PATH_RECEIVE
							}
						}
					}
				}
			}
		}
	}
	pinfo.marshallingOptions = []*bgp.MarshallingOption{{AddPath: addPathOption}}
	p.peers.Store(pkey, pinfo)

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
	pinfo, ok := p.peers.Load(pkey)
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
			nh = helpers.AddrTo6(attr.Value)
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
		if len(rta.asPath) == 0 {
			rta.asn = pkey.asn
		} else {
			rta.asn = rta.asPath[len(rta.asPath)-1]
		}
	}
	if !p.config.CollectASPaths {
		rta.asPath = nil
	}
	rtaHandle := unique.Make(rta.ToComparable())
	nhHandle := unique.Make(nh)

	added := 0
	removed := 0

	// Regular NLRI and withdrawn routes
	if pkey.ptype == bmp.BMP_PEER_TYPE_L3VPN || p.isAcceptedRD(0) {
		// We know we have IPv4 NLRI
		for _, path := range update.NLRI {
			v4UCPrefix, ok := path.NLRI.(*bgp.IPAddrPrefix)
			if !ok {
				continue
			}
			pfx := helpers.PrefixTo6(v4UCPrefix.Prefix)
			added += p.rib.AddPrefix(pfx, route{
				peer: pinfo.reference,
				nlri: unique.Make(nlri{
					family: bgp.RF_IPv4_UC,
					path:   path.ID,
					rd:     pkey.distinguisher,
				}),
				nextHop:    nhHandle,
				attributes: rtaHandle,
				prefixLen:  uint8(pfx.Bits()),
			})
		}
		for _, path := range update.WithdrawnRoutes {
			v4UCPrefix, ok := path.NLRI.(*bgp.IPAddrPrefix)
			if !ok {
				continue
			}
			pfx := helpers.PrefixTo6(v4UCPrefix.Prefix)
			removed += p.rib.RemovePrefix(pfx, route{
				peer: pinfo.reference,
				nlri: unique.Make(nlri{
					family: bgp.RF_IPv4_UC,
					path:   path.ID,
					rd:     pkey.distinguisher,
				}),
			})
		}
	}

	// MP reach and unreach NLRI
	for _, attr := range update.PathAttributes {
		var paths []bgp.PathNLRI
		var family bgp.Family
		switch attr := attr.(type) {
		case *bgp.PathAttributeMpReachNLRI:
			nh = helpers.AddrTo6(attr.Nexthop)
			nhHandle = unique.Make(nh)
			paths = attr.Value
			family = bgp.NewFamily(attr.AFI, attr.SAFI)
		case *bgp.PathAttributeMpUnreachNLRI:
			paths = attr.Value
			family = bgp.NewFamily(attr.AFI, attr.SAFI)
		}
		for _, path := range paths {
			var pfx netip.Prefix
			var rd RD
			switch nlri := path.NLRI.(type) {
			case *bgp.IPAddrPrefix:
				pfx = helpers.PrefixTo6(nlri.Prefix)
				rd = pkey.distinguisher
			case *bgp.LabeledIPAddrPrefix:
				pfx = helpers.PrefixTo6(nlri.Prefix)
				rd = pkey.distinguisher
			case *bgp.LabeledVPNIPAddrPrefix:
				pfx = helpers.PrefixTo6(nlri.Prefix)
				rd = RDFromRouteDistinguisherInterface(nlri.RD)
			case *bgp.EVPNNLRI:
				switch route := nlri.RouteTypeData.(type) {
				case *bgp.EVPNIPPrefixRoute:
					pfx = helpers.PrefixTo6(netip.PrefixFrom(route.IPPrefix, int(route.IPPrefixLength)))
					rd = RDFromRouteDistinguisherInterface(route.RD)
				}
			default:
				p.metrics.ignoredNlri.WithLabelValues(exporterStr, family.String()).Inc()
				continue
			}
			if pkey.ptype != bmp.BMP_PEER_TYPE_L3VPN && !p.isAcceptedRD(rd) {
				continue
			}
			switch attr.(type) {
			case *bgp.PathAttributeMpReachNLRI:
				added += p.rib.AddPrefix(pfx, route{
					peer: pinfo.reference,
					nlri: unique.Make(nlri{
						family: family,
						rd:     rd,
						path:   path.ID,
					}),
					nextHop:    nhHandle,
					attributes: rtaHandle,
					prefixLen:  uint8(pfx.Bits()),
				})
			case *bgp.PathAttributeMpUnreachNLRI:
				removed += p.rib.RemovePrefix(pfx, route{
					peer: pinfo.reference,
					nlri: unique.Make(nlri{
						family: family,
						rd:     rd,
						path:   path.ID,
					}),
				})
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
