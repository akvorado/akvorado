// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package tcp handles TCP listeners.
package tcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"runtime/pprof"
	"time"

	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/inlet/flow/decoder"
	"akvorado/inlet/flow/input"
)

// maxMessageSize is the maximum size of a message.
const maxMessageSize = 65536

// Input represents the state of a TCP listener.
type Input struct {
	r        *reporter.Reporter
	t        tomb.Tomb
	config   Configuration
	splitter decoder.Splitter

	metrics metrics

	address net.Addr       // listening address, for testing purposes
	send    input.SendFunc // function to send to kafka
}

var (
	_ input.Input         = &Input{}
	_ input.Configuration = Configuration{}
)

// New instantiate a new TCP listener from the provided configuration.
func (configuration Configuration) New(r *reporter.Reporter, daemon daemon.Component, dec decoder.Decoder, send input.SendFunc) (input.Input, error) {
	splitter, ok := dec.(decoder.Splitter)
	if !ok {
		return nil, fmt.Errorf("TCP input does not support the %s decoder", dec.Name())
	}
	input := &Input{
		r:        r,
		config:   configuration,
		splitter: splitter,
		send:     send,
	}

	input.initMetrics()

	daemon.Track(&input.t, "inlet/flow/input/tcp")
	return input, nil
}

// Start starts listening to the provided TCP socket and producing flows.
func (in *Input) Start() error {
	in.r.Info().Str("listen", in.config.Listen).Msg("starting TCP input")
	listener, err := net.Listen("tcp", in.config.Listen)
	if err != nil {
		return fmt.Errorf("unable to listen to %v: %w", in.config.Listen, err)
	}
	in.address = listener.Addr()
	in.r.Info().Str("listen", in.address.String()).Msg("TCP input listening")

	ctx := in.t.Context(context.Background())
	errLogger := in.r.Sample(reporter.BurstSampler(time.Minute, 1))
	in.t.Go(func() error {
		labels := pprof.Labels("goroutine", "tcp-listener")
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
				in.serveConnection(conn.(*net.TCPConn))
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

// serveConnection reads messages from a TCP connection until it is
// closed and sends them to Kafka.
func (in *Input) serveConnection(conn *net.TCPConn) {
	listen := in.config.Listen
	remote := conn.RemoteAddr().(*net.TCPAddr)
	source := remote.IP.To16()
	srcIP := remote.IP.String()
	labels := pprof.Labels("goroutine", fmt.Sprintf("tcp-connection-%s", srcIP))
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

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, maxMessageSize), maxMessageSize)
	scanner.Split(in.splitter.Split)
	flow := pb.RawFlow{}
	for scanner.Scan() {
		payload := scanner.Bytes()
		bytesCounter.Add(float64(len(payload)))
		messagesCounter.Inc()

		flow.Reset()
		flow.TimeReceived = uint64(time.Now().Unix())
		flow.Payload = payload
		flow.SourceAddress = source
		in.send(srcIP, &flow)
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, net.ErrClosed) {
		l.Err(err).Msg("unable to read message")
		errorsCounter.Inc()
	}
}

// Stop stops the TCP listener and all the connections.
func (in *Input) Stop() error {
	l := in.r.With().Str("listen", in.config.Listen).Logger()
	defer l.Info().Msg("TCP listener stopped")
	in.t.Kill(nil)
	return in.t.Wait()
}
