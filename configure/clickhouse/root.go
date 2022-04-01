// Package clickhouse handles configuration of the ClickHouse database.
package clickhouse

import (
	"time"

	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

// Component represents the Kafka exporter.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	migrationsDone chan bool
}

// Dependencies define the dependencies of the Kafka exporter.
type Dependencies struct {
	Daemon daemon.Component
	HTTP   *http.Component
}

// New creates a new ClickHouse component.
func New(reporter *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:              reporter,
		d:              &dependencies,
		config:         configuration,
		migrationsDone: make(chan bool),
	}
	if err := c.registerHTTPHandlers(); err != nil {
		return nil, err
	}
	c.d.Daemon.Track(&c.t, "configure/clickhouse")
	return &c, nil
}

// Start the ClickHouse component
func (c *Component) Start() error {
	c.r.Info().Msg("starting ClickHouse component")
	if err := c.migrateDatabase(); err != nil {
		c.r.Warn().Msg("database migration failed, continue in the background")
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
