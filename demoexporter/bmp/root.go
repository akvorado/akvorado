// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package bmp simulates an BMP client
package bmp

import (
	"time"

	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
)

// Component represents the BMP component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	metrics struct {
		connections reporter.Counter
		errors      *reporter.CounterVec
	}
}

// Dependencies define the dependencies of the BMP component.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new BMP component.
func New(r *reporter.Reporter, config Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      r,
		d:      &dependencies,
		config: config,
	}

	c.metrics.connections = c.r.Counter(
		reporter.CounterOpts{
			Name: "connections_total",
			Help: "Number of successful connections to target.",
		},
	)
	c.metrics.errors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Number of unsuccessful connections to target.",
		},
		[]string{"error"},
	)

	if config.Target != "" {
		c.d.Daemon.Track(&c.t, "demo-exporter/bmp")
	}
	return &c, nil
}

// Start starts the BMP component.
func (c *Component) Start() error {
	if c.config.Target != "" {
		c.r.Info().Msg("starting BMP component")
		c.t.Go(func() error {
			for {
				ctx := c.t.Context(nil)
				c.startBMPClient(ctx)
				if !c.t.Alive() {
					return nil
				}
				time.Sleep(c.config.RetryAfter)
			}
		})
	}
	return nil
}

// Stop stops the BMP component.
func (c *Component) Stop() error {
	if c.config.Target != "" {
		defer c.r.Info().Msg("BMP component stopped")
		c.r.Info().Msg("stopping the BMP component")
		c.t.Kill(nil)
		return c.t.Wait()
	}
	return nil
}
