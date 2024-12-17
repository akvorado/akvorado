// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package clickhouse handles flow exports to ClickHouse. This component is
// "inert" and does not track its spawned workers. It is the responsability of
// the dependent component to flush data before shutting down.
package clickhouse

import (
	"akvorado/common/clickhousedb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// Component is the interface for the ClickHouse exporter component.
type Component interface {
	NewWorker(int, *schema.FlowMessage) Worker
}

// realComponent implements the ClickHouse exporter
type realComponent struct {
	r      *reporter.Reporter
	d      *Dependencies
	config Configuration

	metrics metrics
}

// Dependencies defines the dependencies of the ClickHouse exporter
type Dependencies struct {
	ClickHouse *clickhousedb.Component
	Schema     *schema.Component
}

// New creates a new core component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (Component, error) {
	c := realComponent{
		r:      r,
		d:      &dependencies,
		config: configuration,
	}
	c.initMetrics()
	return &c, nil
}
