// Package udp handles UDP listeners.
package udp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"gopkg.in/tomb.v2"

	"akvorado/daemon"
	"akvorado/flow/decoder"
	"akvorado/flow/input"
	"akvorado/reporter"
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
		drops         *reporter.CounterVec
	}

	address net.Addr                    // listening address, for testing purpoese
	ch      chan []*decoder.FlowMessage // channel to send flows to
	decoder decoder.Decoder             // decoder to use
}

// New instantiate a new UDP listener from the provided configuration.
func (configuration *Configuration) New(r *reporter.Reporter, daemon daemon.Component, dec decoder.Decoder) (input.Input, error) {
	input := &Input{
		r:       r,
		config:  configuration,
		ch:      make(chan []*decoder.FlowMessage, configuration.QueueSize),
		decoder: dec,
	}

	input.metrics.bytes = r.CounterVec(
		reporter.CounterOpts{
			Name: "bytes",
			Help: "Bytes received by the application.",
		},
		[]string{"listener", "worker", "sampler"},
	)
	input.metrics.packets = r.CounterVec(
		reporter.CounterOpts{
			Name: "packets",
			Help: "Packets received by the application.",
		},
		[]string{"listener", "worker", "sampler"},
	)
	input.metrics.packetSizeSum = r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "summary_size_bytes",
			Help:       "Summary of packet size.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"listener", "worker", "sampler"},
	)
	input.metrics.errors = r.CounterVec(
		reporter.CounterOpts{
			Name: "errors",
			Help: "Errors while receiving packets by the application.",
		},
		[]string{"listener", "worker"},
	)
	input.metrics.drops = r.CounterVec(
		reporter.CounterOpts{
			Name: "drops",
			Help: "Dropped packets due to internal queue full.",
		},
		[]string{"listener", "worker", "sampler"},
	)

	daemon.Track(&input.t, "flow/input/udp")
	return input, nil
}

// Start starts listening to the provided UDP socket and producing flows.
func (in *Input) Start() (<-chan []*decoder.FlowMessage, error) {
	in.r.Info().Str("listen", in.config.Listen).Msg("starting UDP input")

	// Listen to UDP port
	conns := []*net.UDPConn{}
	for i := 0; i < in.config.Workers; i++ {
		var listenAddr net.Addr
		if in.address != nil {
			// We already are listening on one address, let's
			// listen to the same (useful when using :0).
			listenAddr = in.address
		} else {
			var err error
			listenAddr, err = net.ResolveUDPAddr("udp", in.config.Listen)
			if err != nil {
				return nil, fmt.Errorf("unable to resolve %v: %w", in.config.Listen, err)
			}
		}
		var listenConfig = net.ListenConfig{
			Control: func(network, address string, c syscall.RawConn) error {
				var err error
				c.Control(func(fd uintptr) {
					for _, opt := range []int{unix.SO_REUSEADDR, unix.SO_REUSEPORT} {
						err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, opt, 1)
						if err != nil {
							return
						}
					}
				})
				return err
			},
		}
		pconn, err := listenConfig.ListenPacket(in.t.Context(context.Background()), "udp", listenAddr.String())
		if err != nil {
			return nil, fmt.Errorf("unable to listen to %v: %w", listenAddr, err)
		}
		udpConn := pconn.(*net.UDPConn)
		in.address = udpConn.LocalAddr()

		conns = append(conns, udpConn)
	}

	for i := 0; i < in.config.Workers; i++ {
		workerID := i
		worker := strconv.Itoa(i)
		in.t.Go(func() error {
			payload := make([]byte, 9000)
			listen := in.config.Listen
			l := in.r.With().
				Str("worker", worker).
				Str("listen", listen).
				Logger()
			errLogger := l.Sample(reporter.BurstSampler(time.Minute, 1))
			for {
				size, source, err := conns[workerID].ReadFromUDP(payload)
				if err != nil {
					if errors.Is(err, net.ErrClosed) {
						return nil
					}
					errLogger.Err(err).Msg("unable to receive UDP packet")
					in.metrics.errors.WithLabelValues(listen, worker).Inc()
					continue
				}

				srcIP := source.IP.String()
				flows := in.decoder.Decode(decoder.RawFlow{
					TimeReceived: time.Now(),
					Payload:      payload[:size],
					Source:       source.IP,
				})
				if len(flows) == 0 {
					continue
				}
				select {
				case <-in.t.Dying():
					return nil
				case in.ch <- flows:
					in.metrics.bytes.WithLabelValues(listen, worker, srcIP).
						Add(float64(size))
					in.metrics.packets.WithLabelValues(listen, worker, srcIP).
						Inc()
					in.metrics.packetSizeSum.WithLabelValues(listen, worker, srcIP).
						Observe(float64(size))
				default:
					errLogger.Warn().Msgf("dropping flow due to queue full (size %d)",
						in.config.QueueSize)
					in.metrics.drops.WithLabelValues(listen, worker, srcIP).
						Inc()
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

	return in.ch, nil
}

// Stop stops the UDP listeners
func (in *Input) Stop() error {
	l := in.r.With().Str("listen", in.config.Listen).Logger()
	defer func() {
		close(in.ch)
		l.Info().Msg("UDP listener stopped")
	}()
	in.t.Kill(nil)
	return in.t.Wait()
}
