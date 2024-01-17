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
			Name: "running_migrations",
			Help: "Database migrations in progress.",
		},
	)
	c.metrics.migrationsApplied = c.r.Counter(
		reporter.CounterOpts{
			Name: "migrations_applied_steps_total",
			Help: "Number of migration steps applied",
		},
	)
	c.metrics.migrationsNotApplied = c.r.Counter(
		reporter.CounterOpts{
			Name: "migrations_notapplied_steps_total",
			Help: "Number of migration steps not applied",
		},
	)
}
