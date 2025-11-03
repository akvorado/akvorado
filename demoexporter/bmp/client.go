// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"syscall"
	"time"

	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"github.com/osrg/gobgp/v4/pkg/packet/bmp"
)

// startBMPClient starts the BMP client
func (c *Component) startBMPClient(ctx context.Context) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.config.Target)
	if err != nil {
		c.r.Err(err).Msg("cannot connect to target")
		c.metrics.errors.WithLabelValues("cannot connect").Inc()
		return
	}
	c.metrics.connections.Inc()
	defer conn.Close()

	buf := bytes.NewBuffer([]byte{})
	peerHeader := bmp.NewBMPPeerHeader(
		bmp.BMP_PEER_TYPE_GLOBAL, 0, 0,
		c.config.PeerIP,
		uint32(c.config.PeerASN),
		netip.MustParseAddr("2.2.2.2"),
		0)
	pkt, err := bmp.NewBMPInitiation([]bmp.BMPInfoTLVInterface{
		bmp.NewBMPInfoTLVString(bmp.BMP_INIT_TLV_TYPE_SYS_DESCR, "Fake exporter"),
		bmp.NewBMPInfoTLVString(bmp.BMP_INIT_TLV_TYPE_SYS_NAME, "fake.example.com"),
	}).Serialize()
	if err != nil {
		panic(err)
	}
	buf.Write(pkt)
	om1, err := bgp.NewBGPOpenMessage(c.config.LocalASN, 30, netip.MustParseAddr("1.1.1.1"),
		[]bgp.OptionParameterInterface{
			bgp.NewOptionParameterCapability([]bgp.ParameterCapabilityInterface{
				bgp.NewCapMultiProtocol(bgp.RF_IPv4_UC),
				bgp.NewCapMultiProtocol(bgp.RF_IPv6_UC),
			}),
		},
	)
	if err != nil {
		panic(err)
	}
	om2, err := bgp.NewBGPOpenMessage(c.config.PeerASN, 30, netip.MustParseAddr("2.2.2.2"),
		[]bgp.OptionParameterInterface{
			bgp.NewOptionParameterCapability([]bgp.ParameterCapabilityInterface{
				bgp.NewCapMultiProtocol(bgp.RF_IPv4_UC),
				bgp.NewCapMultiProtocol(bgp.RF_IPv6_UC),
			}),
		},
	)
	if err != nil {
		panic(err)
	}
	pkt, err = bmp.NewBMPPeerUpNotification(*peerHeader, c.config.LocalIP, 179, 47647,
		om1,
		om2,
	).Serialize()
	if err != nil {
		panic(err)
	}
	buf.Write(pkt)

	// Send the routes
	for _, af := range []bgp.Family{bgp.RF_IPv4_UC, bgp.RF_IPv6_UC} {
		var nh netip.Addr
		if af == bgp.RF_IPv4_UC {
			nh = netip.MustParseAddr("192.0.2.1")
		} else {
			nh = netip.MustParseAddr("fe80::1")
		}
		for _, route := range c.config.Routes {
			prefixes := []bgp.PathNLRI{}

			for _, prefix := range route.Prefixes {
				if af == bgp.RF_IPv4_UC && prefix.Addr().Is4() || af == bgp.RF_IPv6_UC && prefix.Addr().Is6() {
					n, err := bgp.NewIPAddrPrefix(prefix)
					if err != nil {
						panic(err)
					}
					prefixes = append(prefixes, bgp.PathNLRI{
						NLRI: n,
					})
				}
			}
			if len(prefixes) == 0 {
				continue
			}
			nlri, err := bgp.NewPathAttributeMpReachNLRI(af, prefixes, nh)
			if err != nil {
				panic(err)
			}
			attrs := []bgp.PathAttributeInterface{
				// bgp.NewPathAttributeNextHop("192.0.2.20"),
				bgp.NewPathAttributeOrigin(1),
				bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{
					bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, route.ASPath),
				}),
				nlri,
			}
			if route.Communities != nil {
				comms := make([]uint32, len(route.Communities))
				for idx, comm := range route.Communities {
					comms[idx] = uint32(comm)
				}
				attrs = append(attrs, bgp.NewPathAttributeCommunities(comms))
			}
			if route.LargeCommunities != nil {
				comms := make([]*bgp.LargeCommunity, len(route.LargeCommunities))
				for idx, comm := range route.LargeCommunities {
					comms[idx] = (*bgp.LargeCommunity)(&comm)
				}
				attrs = append(attrs, bgp.NewPathAttributeLargeCommunities(comms))
			}
			pkt, err = bmp.NewBMPRouteMonitoring(*peerHeader,
				bgp.NewBGPUpdateMessage(nil, attrs, nil)).Serialize()
			if err != nil {
				panic(err)
			}
			buf.Write(pkt)
		}
	}

	// Send the packets on the wire
	if _, err := conn.Write(buf.Bytes()); err != nil {
		c.r.Err(err).Msg("cannot write BMP message to target")
		c.metrics.errors.WithLabelValues("cannot write").Inc()
		return
	}

	// Check if the connection stays up by sending stats messages
	// (we cannot read as remote end may have closed the write
	// side)
	done := make(chan struct{})
	go func() {
		for {
			buf := bytes.NewBuffer([]byte{})
			pkt, err := bmp.NewBMPStatisticsReport(*peerHeader, []bmp.BMPStatsTLVInterface{}).
				Serialize()
			if err != nil {
				panic(err)
			}
			buf.Write(pkt)
			if _, err := conn.Write(buf.Bytes()); err != nil && err != io.EOF && !errors.Is(err, syscall.ECONNRESET) && !errors.Is(err, syscall.EPIPE) {
				c.r.Err(err).Msg("cannot write to remote")
				c.metrics.errors.WithLabelValues("cannot write").Inc()
				close(done)
				return
			} else if err != nil {
				c.r.Info().Msg("remote closed connection")
				c.metrics.errors.WithLabelValues("EOF").Inc()
				close(done)
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(c.config.StatsDelay):
			}
		}
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}
