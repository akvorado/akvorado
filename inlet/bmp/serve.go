// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
	"github.com/osrg/gobgp/v3/pkg/packet/bmp"
)

// serveConnection handle the connection from an exporter.
func (c *Component) serveConnection(conn *net.TCPConn) error {
	remote := conn.RemoteAddr().(*net.TCPAddr)
	exporterIP, _ := netip.AddrFromSlice(remote.IP)
	exporter := netip.AddrPortFrom(exporterIP, uint16(remote.Port))
	exporterStr := exporter.Addr().Unmap().String()
	c.metrics.openedConnections.WithLabelValues(exporterStr).Inc()
	logger := c.r.With().Str("exporter", exporterStr).Logger()
	conn.SetLinger(0)

	// Stop the connection when exiting this method or when dying
	conn.CloseWrite()
	stop := make(chan struct{})
	c.t.Go(func() error {
		select {
		case <-stop:
			logger.Info().Msgf("connection down for %s", exporterStr)
			c.handleConnectionDown(exporter)
		case <-c.t.Dying():
			// No need to clean up
		}
		conn.CloseRead()
		c.metrics.closedConnections.WithLabelValues(exporterStr).Inc()
		return nil
	})
	defer close(stop)

	// Setup TCP keepalive
	if err := conn.SetKeepAlive(true); err != nil {
		c.r.Error().Err(err).Msg("unable to enable keepalive")
		return nil
	}
	if err := conn.SetKeepAlivePeriod(time.Minute); err != nil {
		c.r.Error().Err(err).Msg("unable to set keepalive period")
		return nil
	}

	// Handle panics
	defer func() {
		if r := recover(); r != nil {
			logger.Panic().Str("panic", fmt.Sprintf("%+v", r)).Msg("fatal error while processing BMP messages")
			c.metrics.panics.WithLabelValues(exporterStr).Inc()
		}
	}()

	// Reading from connection
	c.handleConnectionUp(exporter)
	init := false
	header := make([]byte, bmp.BMP_HEADER_SIZE)
	for {
		_, err := io.ReadFull(conn, header)
		if err != nil {
			if c.t.Alive() && err != io.EOF {
				logger.Err(err).Msg("cannot read BMP header")
				c.metrics.errors.WithLabelValues(exporterStr, "cannot read BMP header").Inc()
			}
			return nil
		}
		msg := bmp.BMPMessage{}
		if err := msg.Header.DecodeFromBytes(header); err != nil {
			logger.Err(err).Msg("cannot decode BMP header")
			c.metrics.errors.WithLabelValues(exporterStr, "cannot decode BMP header").Inc()
			return nil
		}
		switch msg.Header.Type {
		case bmp.BMP_MSG_ROUTE_MONITORING:
			msg.Body = &bmp.BMPRouteMonitoring{}
			c.metrics.messages.WithLabelValues(exporterStr, "route-monitoring").Inc()
		case bmp.BMP_MSG_STATISTICS_REPORT:
			// Ignore
			c.metrics.messages.WithLabelValues(exporterStr, "statistics-report").Inc()
		case bmp.BMP_MSG_PEER_DOWN_NOTIFICATION:
			msg.Body = &bmp.BMPPeerDownNotification{}
			c.metrics.messages.WithLabelValues(exporterStr, "peer-down-notification").Inc()
		case bmp.BMP_MSG_PEER_UP_NOTIFICATION:
			msg.Body = &bmp.BMPPeerUpNotification{}
			c.metrics.messages.WithLabelValues(exporterStr, "peer-up-notification").Inc()
		case bmp.BMP_MSG_INITIATION:
			msg.Body = &bmp.BMPInitiation{}
			c.metrics.messages.WithLabelValues(exporterStr, "initiation").Inc()
			init = true
		case bmp.BMP_MSG_TERMINATION:
			msg.Body = &bmp.BMPTermination{}
			c.metrics.messages.WithLabelValues(exporterStr, "termination").Inc()
		case bmp.BMP_MSG_ROUTE_MIRRORING:
			// Ignore
			c.metrics.messages.WithLabelValues(exporterStr, "route-mirroring").Inc()
		default:
			logger.Info().Msgf("unknown BMP message type %d", msg.Header.Type)
			c.metrics.messages.WithLabelValues(exporterStr, "unknown").Inc()
		}

		// First message should be BMP_MSG_INITIATION
		if !init {
			logger.Error().Msg("first message is not `initiation'")
			c.metrics.errors.WithLabelValues(exporterStr, "first message not initiation").Inc()
			return nil
		}

		body := make([]byte, msg.Header.Length-bmp.BMP_HEADER_SIZE)
		_, err = io.ReadFull(conn, body)
		if err != nil {
			if c.t.Alive() {
				logger.Error().Err(err).Msg("cannot read BMP body")
				c.metrics.errors.WithLabelValues(exporterStr, "cannot read BMP body").Inc()
			}
			return nil
		}
		if msg.Body == nil {
			continue
		}

		var marshallingOptions []*bgp.MarshallingOption
		var pkey peerKey
		if msg.Header.Type != bmp.BMP_MSG_INITIATION && msg.Header.Type != bmp.BMP_MSG_TERMINATION {
			if err := msg.PeerHeader.DecodeFromBytes(body); err != nil {
				logger.Error().Err(err).Msg("cannot parse BMP peer header")
				c.metrics.errors.WithLabelValues(exporterStr, "cannot parse BMP peer header").Inc()
				return nil
			}
			body = body[bmp.BMP_PEER_HEADER_SIZE:]
			pkey = peerKeyFromBMPPeerHeader(exporter, &msg.PeerHeader)
			c.mu.RLock()
			if pinfo, ok := c.peers[pkey]; ok {
				marshallingOptions = pinfo.marshallingOptions
			}
			c.mu.RUnlock()
		}

		if err := msg.Body.ParseBody(&msg, body, marshallingOptions...); err != nil {
			logger.Error().Err(err).Msg("cannot parse BMP body")
			c.metrics.errors.WithLabelValues(exporterStr, "cannot parse BMP body").Inc()
			return nil
		}

		// Handle different messages
		switch body := msg.Body.(type) {
		case *bmp.BMPInitiation:
			found := false
			for _, info := range body.Info {
				switch tlv := info.(type) {
				case *bmp.BMPInfoTLVString:
					if tlv.Type == bmp.BMP_INIT_TLV_TYPE_SYS_NAME {
						logger.Info().Str("sysname", tlv.Value).Msg("new connection")
						found = true
					}
				}
			}
			if !found {
				logger.Info().Msg("new connection")
			}
		case *bmp.BMPTermination:
			for _, info := range body.Info {
				switch tlv := info.(type) {
				case *bmp.BMPInfoTLVString:
					logger.Info().Str("reason", tlv.Value).Msg("termination message received")
					return nil
				}
			}
			logger.Info().Msg("termination message received")
			return nil
		case *bmp.BMPPeerUpNotification:
			c.handlePeerUpNotification(pkey, body)
		case *bmp.BMPPeerDownNotification:
			c.handlePeerDownNotification(pkey)
		case *bmp.BMPRouteMonitoring:
			c.handleRouteMonitoring(pkey, body)
		}
	}
}
