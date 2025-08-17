// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package udp handles UDP listeners.
package udp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/inlet/flow/input"
)

// Input represents the state of an UDP listener.
type Input struct {
	r      *reporter.Reporter
	t      tomb.Tomb
	config *Configuration

	metrics struct {
		bytes         *reporter.CounterVec
		packets       *reporter.CounterVec
		packetSizeSum *reporter.SummaryVec
		errors        *reporter.CounterVec
		inDrops       *reporter.CounterVec
	}

	address net.Addr       // listening address, for testing purpoese
	send    input.SendFunc // function to send to kafka
}

// New instantiate a new UDP listener from the provided configuration.
func (configuration *Configuration) New(r *reporter.Reporter, daemon daemon.Component, send input.SendFunc) (input.Input, error) {
	input := &Input{
		r:      r,
		config: configuration,
		send:   send,
	}

	input.metrics.bytes = r.CounterVec(
		reporter.CounterOpts{
			Name: "bytes_total",
			Help: "Bytes received by the application.",
		},
		[]string{"listener", "worker", "exporter"},
	)
	input.metrics.packets = r.CounterVec(
		reporter.CounterOpts{
			Name: "packets_total",
			Help: "Packets received by the application.",
		},
		[]string{"listener", "worker", "exporter"},
	)
	input.metrics.packetSizeSum = r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "size_bytes",
			Help:       "Summary of packet size.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"listener", "worker", "exporter"},
	)
	input.metrics.errors = r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Errors while receiving packets by the application.",
		},
		[]string{"listener", "worker"},
	)
	input.metrics.inDrops = r.CounterVec(
		reporter.CounterOpts{
			Name: "in_dropped_packets_total",
			Help: "Dropped packets due to listen queue full.",
		},
		[]string{"listener", "worker"},
	)

	daemon.Track(&input.t, "inlet/flow/input/udp")
	return input, nil
}

// Start starts listening to the provided UDP socket and producing flows.
func (in *Input) Start() error {
	in.r.Info().Str("listen", in.config.Listen).Msg("starting UDP input")

	// Listen to UDP port
	conns := []*net.UDPConn{}
	for i := range in.config.Workers {
		var listenAddr net.Addr
		if in.address != nil {
			// We already are listening on one address, let's
			// listen to the same (useful when using :0).
			listenAddr = in.address
		} else {
			var err error
			listenAddr, err = net.ResolveUDPAddr("udp", in.config.Listen)
			if err != nil {
				return fmt.Errorf("unable to resolve %v: %w", in.config.Listen, err)
			}
		}
		pconn, err := listenConfig.ListenPacket(in.t.Context(context.Background()), "udp", listenAddr.String())
		if err != nil {
			return fmt.Errorf("unable to listen to %v: %w", listenAddr, err)
		}
		udpConn := pconn.(*net.UDPConn)
		in.address = udpConn.LocalAddr()
		if i == 0 {
			in.r.Info().Str("listen", in.address.String()).Msg("UDP input listening")
		}
		if in.config.ReceiveBuffer > 0 {
			if err := udpConn.SetReadBuffer(int(in.config.ReceiveBuffer)); err != nil {
				// On Linux, this does not trigger an error when we are above net.core.rmem_max.
				in.r.Warn().
					Str("error", err.Error()).
					Str("listen", in.config.Listen).
					Msgf("unable to set requested buffer size (%d bytes)", in.config.ReceiveBuffer)
			}
		}

		conns = append(conns, udpConn)
	}

	for i := range in.config.Workers {
		workerID := i
		worker := strconv.Itoa(i)
		in.t.Go(func() error {
			payload := make([]byte, 9000)
			oob := make([]byte, oobLength)
			flow := pb.RawFlow{}
			listen := in.config.Listen
			l := in.r.With().
				Str("worker", worker).
				Str("listen", listen).
				Logger()
			dying := in.t.Dying()
			errLogger := l.Sample(reporter.BurstSampler(time.Minute, 1))
			for count := 0; ; count++ {
				n, oobn, _, source, err := conns[workerID].ReadMsgUDP(payload, oob)
				if err != nil {
					if errors.Is(err, net.ErrClosed) {
						return nil
					}
					errLogger.Err(err).Msg("unable to receive UDP packet")
					in.metrics.errors.WithLabelValues(listen, worker).Inc()
					continue
				}

				oobMsg, err := parseSocketControlMessage(oob[:oobn])
				if err != nil {
					errLogger.Err(err).Msg("unable to decode UDP control message")
				} else {
					in.metrics.inDrops.WithLabelValues(listen, worker).Add(
						float64(oobMsg.Drops))
				}
				if oobMsg.Received.IsZero() {
					oobMsg.Received = time.Now()
				}

				srcIP := source.IP.String()
				in.metrics.bytes.WithLabelValues(listen, worker, srcIP).
					Add(float64(n))
				in.metrics.packets.WithLabelValues(listen, worker, srcIP).
					Inc()
				in.metrics.packetSizeSum.WithLabelValues(listen, worker, srcIP).
					Observe(float64(n))

				flow.Reset()
				flow.TimeReceived = uint64(oobMsg.Received.Unix())
				flow.Payload = payload[:n]
				flow.SourceAddress = source.IP.To16()
				in.send(srcIP, &flow)

				select {
				case <-dying:
					return nil
				default:
				}
			}
		})

	}

	// Watch for termination and close on dying
	in.t.Go(func() error {
		<-in.t.Dying()
		for _, conn := range conns {
			conn.Close()
		}
		return nil
	})

	return nil
}

// Stop stops the UDP listeners
func (in *Input) Stop() error {
	l := in.r.With().Str("listen", in.config.Listen).Logger()
	defer l.Info().Msg("UDP listener stopped")
	in.t.Kill(nil)
	return in.t.Wait()
}
