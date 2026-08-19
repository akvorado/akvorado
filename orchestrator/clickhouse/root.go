// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package clickhouse handles configuration of the ClickHouse database.
package clickhouse

import (
	"errors"
	"sort"
	"time"

	"github.com/cenkalti/backoff/v7"
	"gopkg.in/tomb.v2"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// Component represents the ClickHouse configurator.
type Component struct {
	r       *reporter.Reporter
	d       *Dependencies
	t       tomb.Tomb
	config  Configuration
	metrics metrics

	shards int // number of shards if in a cluster

	migrationsDone chan bool // closed when migrations are done
	migrationsOnce chan bool // closed after first attempt to migrate

	useZkPathCompatibility bool // set to true if tables are created <= 2026.8.0 and in cluster
}

// Dependencies define the dependencies of the orchestrator.
type Dependencies struct {
	Daemon     daemon.Component
	HTTP       *httpserver.Component
	ClickHouse *clickhousedb.Component
	Schema     *schema.Component
}

// New creates a new ClickHouse component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:              r,
		d:              &dependencies,
		config:         configuration,
		migrationsDone: make(chan bool),
		migrationsOnce: make(chan bool),
	}
	c.initMetrics()

	if err := c.registerHTTPHandlers(); err != nil {
		return nil, err
	}

	// Ensure resolutions are sorted and we have a 0-interval resolution first.
	sort.Slice(c.config.Resolutions, func(i, j int) bool {
		return c.config.Resolutions[i].Interval < c.config.Resolutions[j].Interval
	})
	if len(c.config.Resolutions) == 0 || c.config.Resolutions[0].Interval != 0 {
		return nil, errors.New("resolutions need to be configured, including interval: 0")
	}

	c.d.Daemon.Track(&c.t, "orchestrator/clickhouse")

	return &c, nil
}

// Start the ClickHouse component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting ClickHouse component")

	// stub to prevent tomb dying immediately after migrations are done
	c.t.Go(func() error {
		<-c.t.Dying()
		return nil
	})

	// Database migration
	if c.d.ClickHouse != nil {
		migrationsOnce := false
		c.metrics.migrationsRunning.Set(1)
		c.t.Go(func() error {
			customBackoff := backoff.NewExponentialBackOff()
			customBackoff.InitialInterval = time.Second
			for {
				if !c.config.SkipMigrations {
					c.r.Info().Msg("attempting database migration")
					if err := c.migrateDatabase(); err != nil {
						c.r.Err(err).Msg("database migration error")
					} else {
						return nil
					}
					if !migrationsOnce {
						close(c.migrationsOnce)
						migrationsOnce = true
						customBackoff.Reset()
					}
				}
				next := customBackoff.NextBackOff()
				select {
				case <-c.t.Dying():
					return nil
				case <-time.Tick(next):
				}
			}
		})
	}

	c.r.Info().Msg("ClickHouse component started")
	return nil
}

// Stop stops the ClickHouse component.
func (c *Component) Stop() error {
	c.r.Info().Msg("stopping ClickHouse component")
	defer c.r.Info().Msg("ClickHouse component stopped")
	c.t.Kill(nil)
	return c.t.Wait()
}
