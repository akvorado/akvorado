// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"github.com/osrg/gobgp/v4/pkg/packet/bmp"
)

// bmpMessage is a parsed BMP message header with its raw body bytes,
// passed from the IO goroutine to the processing goroutine.
type bmpMessage struct {
	msg  bmp.BMPMessage
	body []byte
}

// serveConnection handles the connection from an exporter. It reads BMP
// messages from the TCP connection and sends them to a processing goroutine
// via a bounded channel.
func (p *Provider) serveConnection(conn *net.TCPConn, exporter netip.AddrPort, exporterStr string) error {
	p.metrics.openedConnections.WithLabelValues(exporterStr).Inc()
	metricsClosedConnections, _ := p.metrics.closedConnections.GetMetricWithLabelValues(exporterStr)
	logger := p.r.With().Str("exporter", exporterStr).Logger()
	conn.SetLinger(0)

	// Channel for passing messages to the processing goroutine
	ch := make(chan bmpMessage, p.config.MessageBuffer)
	// processingDone channel is closed when all messages have been processed.
	processingDone := make(chan struct{})
	// stop channel is closed when the connection to the station is closed.
	stop := make(chan struct{})

	p.t.Go(func() error {
		select {
		case <-stop:
			<-processingDone
			logger.Info().Msgf("connection down for %s", exporterStr)
			p.handleConnectionDown(exporter)
		case <-p.t.Dying():
			// No need to clean up
		}
		conn.Close()
		metricsClosedConnections.Inc()
		return nil
	})
	defer close(stop)

	// Start processing goroutine
	p.t.Go(func() error {
		defer close(processingDone)
		p.processMessages(conn, exporter, exporterStr, ch)
		return nil
	})
	defer close(ch)

	// Setup TCP keepalive
	if err := conn.SetKeepAlive(true); err != nil {
		p.r.Err(err).Msg("unable to enable keepalive")
		return nil
	}
	if err := conn.SetKeepAlivePeriod(time.Minute); err != nil {
		p.r.Err(err).Msg("unable to set keepalive period")
		return nil
	}

	// Handle panics
	defer func() {
		if r := recover(); r != nil {
			logger.Panic().Str("panic", fmt.Sprintf("%+v", r)).Msg("fatal error while processing BMP messages")
			p.metrics.panics.WithLabelValues(exporterStr).Inc()
		}
	}()

	// Reading from connection
	p.handleConnectionUp(exporter)
	init := false
	header := make([]byte, bmp.BMP_HEADER_SIZE)
	metricRouteMonitoring, _ := p.metrics.messages.GetMetricWithLabelValues(exporterStr, "route-monitoring")
	metricStatisticsReport, _ := p.metrics.messages.GetMetricWithLabelValues(exporterStr, "statistics-report")
	metricsPeerDownNotification, _ := p.metrics.messages.GetMetricWithLabelValues(exporterStr, "peer-down-notification")
	metricsPeerUpNotification, _ := p.metrics.messages.GetMetricWithLabelValues(exporterStr, "peer-up-notification")
	metricsInitiation, _ := p.metrics.messages.GetMetricWithLabelValues(exporterStr, "initiation")
	metricsTermination, _ := p.metrics.messages.GetMetricWithLabelValues(exporterStr, "termination")
	metricsRouteMirroring, _ := p.metrics.messages.GetMetricWithLabelValues(exporterStr, "route-mirroring")
	metricsUnknown, _ := p.metrics.messages.GetMetricWithLabelValues(exporterStr, "unknown")
	metricsFull, _ := p.metrics.messageQueueFull.GetMetricWithLabelValues(exporterStr)
	metricsNotFull, _ := p.metrics.messageQueueNotFull.GetMetricWithLabelValues(exporterStr)

	for {
		_, err := io.ReadFull(conn, header)
		if err != nil {
			if p.t.Alive() && err != io.EOF && !errors.Is(err, net.ErrClosed) {
				logger.Err(err).Msg("cannot read BMP header")
				p.metrics.errors.WithLabelValues(exporterStr, "cannot read BMP header").Inc()
			}
			return nil
		}
		msg := bmp.BMPMessage{}
		if err := msg.Header.DecodeFromBytes(header); err != nil {
			logger.Err(err).Msg("cannot decode BMP header")
			p.metrics.errors.WithLabelValues(exporterStr, "cannot decode BMP header").Inc()
			return nil
		}
		switch msg.Header.Type {
		case bmp.BMP_MSG_ROUTE_MONITORING:
			msg.Body = &bmp.BMPRouteMonitoring{}
			metricRouteMonitoring.Inc()
		case bmp.BMP_MSG_STATISTICS_REPORT:
			// Ignore
			metricStatisticsReport.Inc()
		case bmp.BMP_MSG_PEER_DOWN_NOTIFICATION:
			msg.Body = &bmp.BMPPeerDownNotification{}
			metricsPeerDownNotification.Inc()
		case bmp.BMP_MSG_PEER_UP_NOTIFICATION:
			msg.Body = &bmp.BMPPeerUpNotification{}
			metricsPeerUpNotification.Inc()
		case bmp.BMP_MSG_INITIATION:
			msg.Body = &bmp.BMPInitiation{}
			metricsInitiation.Inc()
			init = true
		case bmp.BMP_MSG_TERMINATION:
			msg.Body = &bmp.BMPTermination{}
			metricsTermination.Inc()
		case bmp.BMP_MSG_ROUTE_MIRRORING:
			// Ignore
			metricsRouteMirroring.Inc()
		default:
			logger.Info().Msgf("unknown BMP message type %d", msg.Header.Type)
			metricsUnknown.Inc()
		}

		// First message should be BMP_MSG_INITIATION
		if !init {
			logger.Error().Msg("first message is not `initiation'")
			p.metrics.errors.WithLabelValues(exporterStr, "first message not initiation").Inc()
			return nil
		}

		body := make([]byte, msg.Header.Length-bmp.BMP_HEADER_SIZE)
		_, err = io.ReadFull(conn, body)
		if err != nil {
			if p.t.Alive() && !errors.Is(err, net.ErrClosed) {
				logger.Err(err).Msg("cannot read BMP body")
				p.metrics.errors.WithLabelValues(exporterStr, "cannot read BMP body").Inc()
			}
			return nil
		}
		if msg.Body == nil || msg.Header.Type == bmp.BMP_MSG_ROUTE_MIRRORING {
			// We do not want to parse route mirroring messages as they can
			// contain malformed BGP messages.
			continue
		}

		select {
		case ch <- bmpMessage{msg: msg, body: body}:
			metricsFull.Inc()
		case <-processingDone:
			// Processsing of messages has exited unexpectedly.
			return nil
		case <-p.t.Dying():
			return nil
		default:
			metricsNotFull.Inc()
			select {
			case ch <- bmpMessage{msg: msg, body: body}:
			case <-processingDone:
				return nil
			case <-p.t.Dying():
				return nil
			}
		}
	}
}

// processMessages reads BMP messages from the channel and processes them.
func (p *Provider) processMessages(conn *net.TCPConn, exporter netip.AddrPort, exporterStr string, ch <-chan bmpMessage) {
	logger := p.r.With().Str("exporter", exporterStr).Logger()
	defer func() {
		if r := recover(); r != nil {
			logger.Panic().Str("panic", fmt.Sprintf("%+v", r)).Msg("fatal error while processing BMP messages")
			p.metrics.panics.WithLabelValues(exporterStr).Inc()
		}
		// We need to close the connection to unstuck the I/O goroutine.
		conn.Close()
	}()

	for m := range ch {
		msg := m.msg
		body := m.body

		var marshallingOptions []*bgp.MarshallingOption
		var pkey peerKey
		if msg.Header.Type != bmp.BMP_MSG_INITIATION && msg.Header.Type != bmp.BMP_MSG_TERMINATION {
			if err := msg.PeerHeader.DecodeFromBytes(body); err != nil {
				logger.Err(err).Msg("cannot parse BMP peer header")
				p.metrics.errors.WithLabelValues(exporterStr, "cannot parse BMP peer header").Inc()
				return
			}
			body = body[bmp.BMP_PEER_HEADER_SIZE:]
			pkey = peerKeyFromBMPPeerHeader(exporter, &msg.PeerHeader)
			p.mu.RLock()
			if pinfo, ok := p.peers[pkey]; ok {
				marshallingOptions = pinfo.marshallingOptions
			}
			p.mu.RUnlock()
		}

		if err := msg.Body.ParseBody(&msg, body, marshallingOptions...); err != nil {
			msgError, ok := err.(*bgp.MessageError)
			if ok {
				switch msgError.ErrorHandling {
				case bgp.ERROR_HANDLING_SESSION_RESET:
					p.metrics.ignored.WithLabelValues(exporterStr, "session-reset").Inc()
					continue
				case bgp.ERROR_HANDLING_AFISAFI_DISABLE:
					p.metrics.ignored.WithLabelValues(exporterStr, "afi-safi").Inc()
					continue
				case bgp.ERROR_HANDLING_TREAT_AS_WITHDRAW:
					p.metrics.ignored.WithLabelValues(exporterStr, "treat-as-withdraw").Inc()
					continue
				case bgp.ERROR_HANDLING_ATTRIBUTE_DISCARD:
					// Optional attribute, let's handle it
				case bgp.ERROR_HANDLING_NONE:
					p.metrics.ignored.WithLabelValues(exporterStr, "none").Inc()
					continue
				}
			} else {
				logger.Err(err).Msg("cannot parse BMP body")
				p.metrics.errors.WithLabelValues(exporterStr, "cannot parse BMP body").Inc()
				return
			}
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
					return
				}
			}
			logger.Info().Msg("termination message received")
			return
		case *bmp.BMPPeerUpNotification:
			p.handlePeerUpNotification(pkey, body)
		case *bmp.BMPPeerDownNotification:
			p.handlePeerDownNotification(pkey)
		case *bmp.BMPRouteMonitoring:
			p.handleRouteMonitoring(pkey, body)
		}
	}
}
