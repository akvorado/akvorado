// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package clickhouse handles configuration of the ClickHouse database.
package clickhouse

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"akvorado/common/remotedatasource"

	"github.com/cenkalti/backoff/v4"
	"gopkg.in/tomb.v2"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/orchestrator/geoip"
)

// Component represents the ClickHouse configurator.
type Component struct {
	r       *reporter.Reporter
	d       *Dependencies
	t       tomb.Tomb
	config  Configuration
	metrics metrics

	shards int // number of shards if in a cluster

	migrationsDone        chan bool // closed when migrations are done
	migrationsOnce        chan bool // closed after first attempt to migrate
	networkSourcesFetcher *remotedatasource.Component[externalNetworkAttributes]
	networkSources        map[string][]externalNetworkAttributes
	networkSourcesLock    sync.RWMutex

	networksCSVReady      chan bool // close when networks.csv was generated once
	networksCSVUpdateChan chan bool // channel to write to to request updates
	networksCSVFile       *os.File
	networksCSVLock       sync.Mutex
}

// Dependencies define the dependencies of the orchestrator.
type Dependencies struct {
	Daemon     daemon.Component
	HTTP       *httpserver.Component
	ClickHouse *clickhousedb.Component
	Schema     *schema.Component
	GeoIP      *geoip.Component
}

// New creates a new ClickHouse component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:                     r,
		d:                     &dependencies,
		config:                configuration,
		migrationsDone:        make(chan bool),
		migrationsOnce:        make(chan bool),
		networkSources:        make(map[string][]externalNetworkAttributes),
		networksCSVReady:      make(chan bool),
		networksCSVUpdateChan: make(chan bool, 1),
	}
	var err error
	c.networkSourcesFetcher, err = remotedatasource.New[externalNetworkAttributes](
		r, c.UpdateSource, "network_source", configuration.NetworkSources)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize remote data source fetcher component: %w", err)
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
	}

	// Network sources update
	if err := c.networkSourcesFetcher.Start(); err != nil {
		return fmt.Errorf("unable to start network sources fetcher component: %w", err)
	}

	// GeoIP updates
	notifyChan := c.d.GeoIP.Notify()
	c.t.Go(func() error {
		c.r.Info().Msg("starting GeoIP refresher")
		for {
			select {
			case <-c.t.Dying():
				return nil
			case <-notifyChan:
				c.triggerNetworksCSVRefresh()
			}
		}
	})

	// networks.csv refresh
	c.t.Go(func() error {
		c.networksCSVRefresher()

		c.r.Debug().Msg("remove networks.csv")
		c.networksCSVLock.Lock()
		if c.networksCSVFile != nil {
			c.networksCSVFile.Close()
			os.Remove(c.networksCSVFile.Name())
		}
		c.networksCSVLock.Unlock()

		return nil
	})

	c.r.Info().Msg("ClickHouse component started")
	return nil
}

// Stop stops the ClickHouse component.
func (c *Component) Stop() error {
	c.r.Info().Msg("stopping ClickHouse component")
	defer c.r.Info().Msg("ClickHouse component stopped")
	c.t.Kill(nil)
	c.networkSourcesFetcher.Stop()
	return c.t.Wait()
}
