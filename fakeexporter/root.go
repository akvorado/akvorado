// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package fakeexporter simulates an exporter (NetFlow and SNMP)
package fakeexporter

import (
	"akvorado/common/reporter"
	"akvorado/fakeexporter/flows"
	"akvorado/fakeexporter/snmp"
)

// Component represents the fake exporter service.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	config Configuration
}

// Dependencies define the dependencies of the fake exporter service.
type Dependencies struct {
	SNMP  *snmp.Component
	Flows *flows.Component
}

// New creates a new fake exporter service.
func New(r *reporter.Reporter, config Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      r,
		d:      &dependencies,
		config: config,
	}
	return &c, nil
}
