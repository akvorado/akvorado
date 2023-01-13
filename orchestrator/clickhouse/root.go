// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package clickhouse handles configuration of the ClickHouse database.
package clickhouse

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/itchyny/gojq"
	"gopkg.in/tomb.v2"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

// Component represents the ClickHouse configurator.
type Component struct {
	r       *reporter.Reporter
	d       *Dependencies
	t       tomb.Tomb
	config  Configuration
	metrics metrics

	migrationsDone      chan bool // closed when migrations are done
	migrationsOnce      chan bool // closed after first attempt to migrate
	networkSourcesReady chan bool // closed when all network sources are ready
	networkSourcesLock  sync.RWMutex
	networkSources      map[string][]externalNetworkAttributes
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
		r:                   r,
		d:                   &dependencies,
		config:              configuration,
		migrationsDone:      make(chan bool),
		migrationsOnce:      make(chan bool),
		networkSourcesReady: make(chan bool),
		networkSources:      make(map[string][]externalNetworkAttributes),
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
		c.config.Resolutions = append([]ResolutionConfiguration{}, c.config.Resolutions...)
	}

	return &c, nil
}

// Start the ClickHouse component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting ClickHouse component")

	// Database migration
	migrationsOnce := false
	c.metrics.migrationsRunning.Set(1)
	c.t.Go(func() error {
		customBackoff := backoff.NewExponentialBackOff()
		customBackoff.MaxElapsedTime = 0
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

	// Network sources update
	var notReadySources sync.WaitGroup
	notReadySources.Add(len(c.config.NetworkSources))
	go func() {
		notReadySources.Wait()
		close(c.networkSourcesReady)
	}()
	for name, source := range c.config.NetworkSources {
		if source.Transform.Query == nil {
			source.Transform.Query, _ = gojq.Parse(".")
		}
		if source.Timeout == 0 {
			source.Timeout = time.Minute
		}
		name := name
		source := source
		c.t.Go(func() error {
			c.metrics.networkSourceCount.WithLabelValues(name).Set(0)
			newRetryTicker := func() *backoff.Ticker {
				customBackoff := backoff.NewExponentialBackOff()
				customBackoff.MaxElapsedTime = 0
				customBackoff.MaxInterval = source.Interval
				customBackoff.InitialInterval = source.Interval / 10
				if customBackoff.InitialInterval > time.Second {
					customBackoff.InitialInterval = time.Second
				}
				return backoff.NewTicker(customBackoff)
			}
			newRegularTicker := func() *time.Ticker {
				return time.NewTicker(source.Interval)
			}
			retryTicker := newRetryTicker()
			regularTicker := newRegularTicker()
			regularTicker.Stop()
			success := false
			ready := false
			defer func() {
				if !success {
					retryTicker.Stop()
				} else {
					regularTicker.Stop()
				}
				if !ready {
					notReadySources.Done()
				}
			}()
			for {
				ctx, cancel := context.WithTimeout(c.t.Context(nil), source.Timeout)
				count, err := c.updateNetworkSource(ctx, name, source)
				cancel()
				if err == nil {
					c.metrics.networkSourceUpdates.WithLabelValues(name).Inc()
					c.metrics.networkSourceCount.WithLabelValues(name).Set(float64(count))
				} else {
					c.metrics.networkSourceErrors.WithLabelValues(name, err.Error()).Inc()
				}
				if err == nil && !ready {
					ready = true
					notReadySources.Done()
					c.r.Debug().Str("name", name).Msg("source ready")
				}
				if err == nil && !success {
					// On success, change the timer to a regular timer interval
					retryTicker.Stop()
					retryTicker.C = nil
					regularTicker = newRegularTicker()
					success = true
					c.r.Debug().Str("name", name).Msg("switch to regular polling")
				} else if err != nil && success {
					// On failure, switch to the retry ticker
					regularTicker.Stop()
					retryTicker = newRetryTicker()
					success = false
					c.r.Debug().Str("name", name).Msg("switch to retry polling")
				}
				select {
				case <-c.t.Dying():
					return nil
				case <-retryTicker.C:
				case <-regularTicker.C:
				}
			}
		})
	}
	return nil
}

// Stop stops the ClickHouse component.
func (c *Component) Stop() error {
	c.r.Info().Msg("stopping ClickHouse component")
	defer c.r.Info().Msg("ClickHouse component stopped")
	c.t.Kill(nil)
	return c.t.Wait()
}
