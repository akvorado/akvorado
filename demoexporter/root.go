// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package demoexporter simulates an exporter (NetFlow and SNMP)
package demoexporter

import (
	"akvorado/common/reporter"
	"akvorado/demoexporter/bmp"
	"akvorado/demoexporter/flows"
	"akvorado/demoexporter/snmp"
)

// Component represents the demo exporter service.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	config Configuration
}

// Dependencies define the dependencies of the demo exporter service.
type Dependencies struct {
	SNMP  *snmp.Component
	BMP   *bmp.Component
	Flows *flows.Component
}

// New creates a new demo exporter service.
func New(r *reporter.Reporter, config Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      r,
		d:      &dependencies,
		config: config,
	}
	return &c, nil
}
