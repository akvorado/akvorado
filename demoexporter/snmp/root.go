// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package snmp simulates an SNMP agent
package snmp

import (
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
)

// Component represents the SNMP component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	snmpPort int
	metrics  struct {
		requests *reporter.CounterVec
	}
}

// Dependencies define the dependencies of the SNMP component.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new SNMP component.
func New(r *reporter.Reporter, config Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      r,
		d:      &dependencies,
		config: config,
	}

	c.metrics.requests = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "requests_total",
			Help: "Number of SNMP requests handled.",
		},
		[]string{"oid"},
	)

	c.d.Daemon.Track(&c.t, "demo-exporter/snmp")
	return &c, nil
}

// Start starts the SNMP component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting SNMP component")
	return c.startSNMPServer()
}

// Stop stops the SNMP component.
func (c *Component) Stop() error {
	defer c.r.Info().Msg("SNMP component stopped")
	c.r.Info().Msg("stopping the SNMP component")
	c.t.Kill(nil)
	return c.t.Wait()
}
