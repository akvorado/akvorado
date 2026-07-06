// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package syslogcgnat handles CGNAT mapping ingestion from syslog over UDP.
package syslogcgnat

import (
	"errors"
	"fmt"
	"net"

	"gopkg.in/tomb.v2"

	commoncgnat "akvorado/common/cgnat"
	"akvorado/common/daemon"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/inlet/flow/input"
)

// Input represents one syslog CGNAT listener.
type Input struct {
	r      *reporter.Reporter
	t      tomb.Tomb
	config Configuration
	send   input.SendFunc

	metrics struct {
		messages *reporter.CounterVec
		events   *reporter.CounterVec
		errors   *reporter.CounterVec
	}
}

var (
	_ input.Input         = &Input{}
	_ input.Configuration = Configuration{}
)

// New instantiates a new syslog CGNAT input from configuration.
func (configuration Configuration) New(r *reporter.Reporter, daemon daemon.Component, send input.SendFunc) (input.Input, error) {
	in := &Input{
		r:      r,
		config: configuration,
		send:   send,
	}

	in.metrics.messages = r.CounterVec(
		reporter.CounterOpts{Name: "syslog_messages_total", Help: "Syslog messages received by CGNAT input."},
		[]string{"listener", "source"},
	)
	in.metrics.events = r.CounterVec(
		reporter.CounterOpts{Name: "events_total", Help: "CGNAT events extracted from syslog."},
		[]string{"listener", "operation"},
	)
	in.metrics.errors = r.CounterVec(
		reporter.CounterOpts{Name: "errors_total", Help: "Errors while parsing CGNAT syslog."},
		[]string{"listener"},
	)

	daemon.Track(&in.t, "inlet/flow/input/syslogcgnat")
	return in, nil
}

// Start starts listening for syslog CGNAT mapping lines.
func (in *Input) Start() error {
	in.r.Info().Str("listen", in.config.Listen).Msg("starting syslog CGNAT input")

	conn, err := net.ListenPacket("udp", in.config.Listen)
	if err != nil {
		return fmt.Errorf("unable to listen to %s: %w", in.config.Listen, err)
	}
	if udpConn, ok := conn.(*net.UDPConn); ok && in.config.ReceiveBuffer > 0 {
		if err := udpConn.SetReadBuffer(int(in.config.ReceiveBuffer)); err != nil {
			in.r.Warn().Err(err).Str("listen", in.config.Listen).Msg("unable to set read buffer")
		}
	}

	in.t.Go(func() error {
		<-in.t.Dying()
		_ = conn.Close()
		return nil
	})

	in.t.Go(func() error {
		defer conn.Close()
		payload := make([]byte, 8192)
		flow := pb.RawFlow{}
		for {
			n, source, err := conn.ReadFrom(payload)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return nil
				}
				in.metrics.errors.WithLabelValues(in.config.Listen).Inc()
				continue
			}

			sourceUDP, ok := source.(*net.UDPAddr)
			if !ok {
				in.metrics.errors.WithLabelValues(in.config.Listen).Inc()
				continue
			}
			sourceIP := sourceUDP.IP.String()
			in.metrics.messages.WithLabelValues(in.config.Listen, sourceIP).Inc()

			event, err := commoncgnat.ParseSyslogLine(string(payload[:n]))
			if err != nil {
				in.metrics.errors.WithLabelValues(in.config.Listen).Inc()
				continue
			}
			eventPayload, err := commoncgnat.Encode(event)
			if err != nil {
				in.metrics.errors.WithLabelValues(in.config.Listen).Inc()
				continue
			}

			opName := "allocate"
			if event.Operation == commoncgnat.OperationFree {
				opName = "free"
			}
			in.metrics.events.WithLabelValues(in.config.Listen, opName).Inc()

			flow.Reset()
			flow.TimeReceived = uint64(event.Timestamp.Unix())
			flow.Payload = eventPayload
			flow.SourceAddress = sourceUDP.IP.To16()
			in.send(sourceIP, &flow)

			select {
			case <-in.t.Dying():
				return nil
			default:
			}
		}
	})

	return nil
}

// Stop stops the input.
func (in *Input) Stop() error {
	in.t.Kill(nil)
	return in.t.Wait()
}
