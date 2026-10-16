// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package sctp handles SCTP listeners.
package sctp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/georgeyanev/go-sctp"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/inlet/flow/decoder"
	"akvorado/inlet/flow/input"
)

// Input represents the state of an SCTP listener.
type Input struct {
	r      *reporter.Reporter
	t      tomb.Tomb
	config Configuration

	metrics metrics

	address net.Addr       // listening address, for testing purposes
	send    input.SendFunc // function to send to kafka
}

var (
	_ input.Input         = &Input{}
	_ input.Configuration = Configuration{}
)

// New instantiate a new SCTP listener from the provided configuration.
func (configuration Configuration) New(r *reporter.Reporter, daemon daemon.Component, _ decoder.Decoder, send input.SendFunc) (input.Input, error) {
	input := &Input{
		r:      r,
		config: configuration,
		send:   send,
	}

	input.initMetrics()

	daemon.Track(&input.t, "inlet/flow/input/sctp")
	return input, nil
}

// Start starts listening to the provided SCTP socket and producing flows.
func (in *Input) Start() error {
	in.r.Info().Str("listen", in.config.Listen).Msg("starting SCTP input")
	lc := sctp.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) error {
			if err := enablePRSCTP(c); err != nil {
				in.r.Warn().Err(err).Str("listen", in.config.Listen).Msg("cannot enable PR-SCTP")
			}
			if err := subscribePartialDelivery(c); err != nil {
				return fmt.Errorf("unable to subscribe to SCTP partial delivery events: %w", err)
			}
			return nil
		},
	}
	listener, err := lc.Listen("sctp", in.config.Listen)
	if err != nil {
		return fmt.Errorf("unable to listen to %v: %w", in.config.Listen, err)
	}
	in.address = listener.Addr()
	in.r.Info().Str("listen", in.address.String()).Msg("SCTP input listening")

	ctx := in.t.Context(context.Background())
	errLogger := in.r.Sample(reporter.BurstSampler(time.Minute, 1))
	in.t.Go(func() error {
		labels := pprof.Labels("goroutine", "sctp-listener")
		pprof.SetGoroutineLabels(pprof.WithLabels(context.Background(), labels))
		for {
			conn, err := helpers.Accept(ctx, listener, func(err error, delay time.Duration) {
				errLogger.Err(err).
					Str("listen", in.config.Listen).
					Msgf("cannot accept new connection, retrying in %s", delay)
			})
			if err != nil {
				return nil
			}
			in.t.Go(func() error {
				in.serveConnection(conn.(*sctp.SCTPConn))
				return nil
			})
		}
	})
	in.t.Go(func() error {
		<-ctx.Done()
		listener.Close()
		return nil
	})
	return nil
}

// serveConnection reads messages from an SCTP association and sends them to
// Kafka.
func (in *Input) serveConnection(conn *sctp.SCTPConn) {
	listen := in.config.Listen
	var source net.IP
	if remote, ok := conn.RemoteAddr().(*sctp.SCTPAddr); ok && remote != nil && len(remote.IPAddrs) > 0 {
		source = remote.IPAddrs[0].IP
	} else {
		in.r.Error().Str("listen", listen).Msg("cannot get the address of the SCTP peer")
		conn.Close()
		return
	}
	srcIP := source.String()
	source = source.To16()
	labels := pprof.Labels("goroutine", fmt.Sprintf("sctp-connection-%s", srcIP))
	pprof.SetGoroutineLabels(pprof.WithLabels(context.Background(), labels))
	l := in.r.With().
		Str("listen", listen).
		Str("exporter", srcIP).
		Logger()
	l.Info().Msg("new connection")
	in.metrics.openedConnections.WithLabelValues(listen, srcIP).Inc()
	connectionsGauge := in.metrics.connections.WithLabelValues(listen, srcIP)
	connectionsGauge.Inc()
	bytesCounter := in.metrics.bytes.WithLabelValues(listen, srcIP)
	messagesCounter := in.metrics.messages.WithLabelValues(listen, srcIP)
	errorsCounter := in.metrics.errors.WithLabelValues(listen, srcIP)
	abortedCounter := in.metrics.droppedMessages.WithLabelValues(listen, srcIP, "aborted")
	tooLongCounter := in.metrics.droppedMessages.WithLabelValues(listen, srcIP, "too-long")

	stop := make(chan struct{})
	defer func() {
		close(stop)
		in.metrics.closedConnections.WithLabelValues(listen, srcIP).Inc()
		connectionsGauge.Dec()
		l.Info().Msg("connection closed")
		conn.Close()
	}()
	in.t.Go(func() error {
		select {
		case <-stop:
		case <-in.t.Dying():
			conn.Close()
		}
		return nil
	})

	reader := newMessageReader(
		func(b []byte) (int, int, error) {
			n, _, flags, err := conn.ReadMsgExt(b, nil)
			return n, flags, err
		},
		func(reason error) {
			if errors.Is(reason, errMessageTooLong) {
				tooLongCounter.Inc()
			} else {
				abortedCounter.Inc()
			}
		})
	flow := pb.RawFlow{}
	for {
		payload, err := reader.next()
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				l.Err(err).Msg("unable to read message")
				errorsCounter.Inc()
			}
			return
		}
		bytesCounter.Add(float64(len(payload)))
		messagesCounter.Inc()

		flow.Reset()
		flow.TimeReceived = uint64(time.Now().Unix())
		flow.Payload = payload
		flow.SourceAddress = source
		in.send(srcIP, &flow)
	}
}

// Stop stops the SCTP listener and all the connections.
func (in *Input) Stop() error {
	l := in.r.With().Str("listen", in.config.Listen).Logger()
	defer l.Info().Msg("SCTP listener stopped")
	in.t.Kill(nil)
	return in.t.Wait()
}
