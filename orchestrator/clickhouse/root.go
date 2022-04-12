// Package clickhouse handles configuration of the ClickHouse database.
package clickhouse

import (
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
		migrationsRunning reporter.Gauge
		migrationsVersion reporter.Gauge
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
	c.d.Daemon.Track(&c.t, "orchestrator/clickhouse")

	c.metrics.migrationsRunning = c.r.Gauge(
		reporter.GaugeOpts{
			Name: "migrations_running",
			Help: "Database migrations in progress.",
		},
	)
	c.metrics.migrationsVersion = c.r.Gauge(
		reporter.GaugeOpts{
			Name: "migrations_version",
			Help: "Current version for migrations.",
		},
	)

	return &c, nil
}

// Start the ClickHouse component
func (c *Component) Start() error {
	c.r.Info().Msg("starting ClickHouse component")
	c.metrics.migrationsRunning.Set(1)
	if err := c.migrateDatabase(); err != nil {
		c.r.Warn().Msgf("database migration failed %s, continue in the background", err.Error())
		c.t.Go(func() error {
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
	}
	c.t.Go(func() error {
		// We need at least one goroutine.
		select {
		case <-c.t.Dying():
			return nil
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
