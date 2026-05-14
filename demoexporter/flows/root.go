// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package flows simulates a NetFlow exporter
package flows

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
)

// Component represents the flows component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration
	target atomic.Pointer[string] // make testing easier

	metrics struct {
		sent   *reporter.CounterVec
		errors *reporter.CounterVec
	}
}

// Dependencies define the dependencies of the flows component.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new flows component.
func New(r *reporter.Reporter, config Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      r,
		d:      &dependencies,
		config: config,
	}
	c.target.Store(&config.Target)

	c.metrics.sent = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "sent_packets_total",
			Help: "Number of packets sent.",
		},
		[]string{"type"},
	)
	c.metrics.errors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Number of transmission errors.",
		},
		[]string{"error"},
	)

	c.d.Daemon.Track(&c.t, "demo-exporter/flows")
	return &c, nil
}

// Start starts the flows component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting flows component")
	target := *c.target.Load()
	conn, err := net.Dial("udp", target)
	if err != nil {
		return fmt.Errorf("cannot create socket to %q: %w", target, err)
	}

	sequenceNumber := uint32(1)
	start := time.Now()
	errLogger := c.r.Sample(reporter.BurstSampler(time.Minute, 10))

	c.t.Go(func() error {
		defer conn.Close()
		ctx := c.t.Context(context.Background())
		elapsedSeconds := 0
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		// redial re-resolves the target and swaps the connection when the
		// resolved address changed.
		redial := func() {
			target := *c.target.Load()
			newConn, err := net.Dial("udp", target)
			if err != nil {
				c.metrics.errors.WithLabelValues("redial").Inc()
				c.r.Err(err).Msgf("cannot redial %q", target)
				return
			}
			if newConn.RemoteAddr().String() == conn.RemoteAddr().String() {
				// No need to redial as the target did not change.
				newConn.Close()
				return
			}
			c.r.Info().Msgf("target %q resolved to new address %s",
				target, newConn.RemoteAddr())
			conn.Close()
			conn = newConn
		}
		transmit := func(kind string, payloads <-chan []byte) {
			for payload := range payloads {
				sequenceNumber++
				if _, err := conn.Write(payload); err != nil {
					c.metrics.errors.WithLabelValues("cannot write").Inc()
					errLogger.Err(err).Msg("unable to send UDP payload")
				} else {
					c.metrics.sent.WithLabelValues(kind).Inc()
				}
			}
		}
		for {
			select {
			case <-c.t.Dying():
				return nil
			case now := <-ticker.C:
				if elapsedSeconds%30 == 0 {
					redial()
					transmit("template",
						getNetFlowTemplates(ctx, sequenceNumber,
							c.config.SamplingRate,
							start, now))
				}
				elapsedSeconds++
				flows := generateFlows(c.config.Flows, c.config.Seed, now)
				transmit("data",
					getNetFlowData(ctx, flows, sequenceNumber,
						start, now))
			}
		}
	})
	return nil
}

// Stop stops the flows component.
func (c *Component) Stop() error {
	defer c.r.Info().Msg("flows component stopped")
	c.r.Info().Msg("stopping the flows component")
	c.t.Kill(nil)
	return c.t.Wait()
}
