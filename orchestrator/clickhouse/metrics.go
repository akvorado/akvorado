// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import "akvorado/common/reporter"

type metrics struct {
	migrationsRunning    reporter.Gauge
	migrationsApplied    reporter.Counter
	migrationsNotApplied reporter.Counter

	networkSourceUpdates *reporter.CounterVec
	networkSourceErrors  *reporter.CounterVec
	networkSourceCount   *reporter.GaugeVec
}

func (c *Component) initMetrics() {
	c.metrics.migrationsRunning = c.r.Gauge(
		reporter.GaugeOpts{
			Name: "migrations_running",
			Help: "Database migrations in progress.",
		},
	)
	c.metrics.migrationsApplied = c.r.Counter(
		reporter.CounterOpts{
			Name: "migrations_applied_steps",
			Help: "Number of migration steps applied",
		},
	)
	c.metrics.migrationsNotApplied = c.r.Counter(
		reporter.CounterOpts{
			Name: "migrations_notapplied_steps",
			Help: "Number of migration steps not applied",
		},
	)

	c.metrics.networkSourceUpdates = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "network_source_updates_total",
			Help: "Number of successful updates for a network source",
		},
		[]string{"source"},
	)
	c.metrics.networkSourceErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "network_source_errors_total",
			Help: "Number of failed updates for a network source",
		},
		[]string{"source", "error"},
	)
	c.metrics.networkSourceCount = c.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "network_source_networks_total",
			Help: "Number of networks imported from a given source",
		},
		[]string{"source"},
	)
}
