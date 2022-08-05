// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package clickhouse handles configuration of the ClickHouse database.
package clickhouse

import (
	"sort"
	"time"

	"gopkg.in/tomb.v2"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

// Component represents the ClickHouse configurator.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	metrics struct {
		migrationsRunning    reporter.Gauge
		migrationsApplied    reporter.Counter
		migrationsNotApplied reporter.Counter
		migrationsVersion    reporter.Gauge
	}

	migrationsDone chan bool
}

// Dependencies define the dependencies of the ClickHouse configurator.
type Dependencies struct {
	Daemon     daemon.Component
	HTTP       *http.Component
	ClickHouse *clickhousedb.Component
}

// New creates a new ClickHouse component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:              r,
		d:              &dependencies,
		config:         configuration,
		migrationsDone: make(chan bool),
	}
	if err := c.registerHTTPHandlers(); err != nil {
		return nil, err
	}
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
	c.metrics.migrationsVersion = c.r.Gauge(
		reporter.GaugeOpts{
			Name: "migrations_version",
			Help: "Current version for migrations.",
		},
	)

	// Ensure resolutions are sorted and we have a 0-interval resolution first.
	sort.Slice(c.config.Resolutions, func(i, j int) bool {
		return c.config.Resolutions[i].Interval < c.config.Resolutions[j].Interval
	})
	if len(c.config.Resolutions) == 0 || c.config.Resolutions[0].Interval != 0 {
		c.config.Resolutions = append([]ResolutionConfiguration{}, c.config.Resolutions...)
	}

	c.d.Daemon.Track(&c.t, "orchestrator/clickhouse")
	return &c, nil
}

// Start the ClickHouse component
func (c *Component) Start() error {
	c.r.Info().Msg("starting ClickHouse component")
	c.metrics.migrationsRunning.Set(1)
	c.t.Go(func() error {
		if err := c.migrateDatabase(); err == nil {
			return nil
		}
		for {
			select {
			case <-c.t.Dying():
				return nil
			case <-time.After(time.Minute):
				c.r.Info().Msg("attempting database migration")
				if err := c.migrateDatabase(); err == nil {
					return nil
				}
			}
		}
	})
	return nil
}

// Stop stops the ClickHouse component
func (c *Component) Stop() error {
	c.r.Info().Msg("stopping ClickHouse component")
	defer c.r.Info().Msg("ClickHouse component stopped")
	c.t.Kill(nil)
	return c.t.Wait()
}
