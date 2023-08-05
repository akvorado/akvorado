// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package geoip provides ASN and country for GeoIP addresses.
package geoip

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
)

// Component represents the GeoIP component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	db struct {
		geo atomic.Pointer[geoDatabase]
		asn atomic.Pointer[geoDatabase]
	}
	metrics struct {
		databaseRefresh *reporter.CounterVec
		databaseHit     *reporter.CounterVec
		databaseMiss    *reporter.CounterVec
	}
}

// Dependencies define the dependencies of the GeoIP component.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new GeoIP component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      r,
		d:      &dependencies,
		config: configuration,
	}
	if c.config.GeoDatabase != "" {
		c.config.GeoDatabase = filepath.Clean(c.config.GeoDatabase)
	}
	if c.config.ASNDatabase != "" {
		c.config.ASNDatabase = filepath.Clean(c.config.ASNDatabase)
	}
	c.d.Daemon.Track(&c.t, "inlet/geoip")
	c.metrics.databaseRefresh = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "db_refresh_total",
			Help: "Refresh event for a GeoIP database.",
		},
		[]string{"database"},
	)
	c.metrics.databaseHit = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "db_hits_total",
			Help: "Number of hits for a GeoIP database.",
		},
		[]string{"database"},
	)
	c.metrics.databaseMiss = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "db_misses_total",
			Help: "Number of misses for a GeoIP database.",
		},
		[]string{"database"},
	)
	return &c, nil
}

// Start starts the GeoIP component.
func (c *Component) Start() error {
	if err := c.openDatabase("geo", c.config.GeoDatabase, &c.db.geo); err != nil && !c.config.Optional {
		return err
	}
	if err := c.openDatabase("asn", c.config.ASNDatabase, &c.db.asn); err != nil && !c.config.Optional {
		return err
	}
	if c.db.geo.Load() == nil && c.db.asn.Load() == nil {
		c.r.Warn().Msg("skipping GeoIP component: no database specified")
		return nil
	}

	c.r.Info().Msg("starting GeoIP component")

	// Watch for modifications
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		c.r.Err(err).Msg("cannot setup watcher for GeoIP databases")
		return fmt.Errorf("cannot setup watcher: %w", err)
	}
	dirs := map[string]struct{}{}
	if c.config.GeoDatabase != "" {
		dirs[filepath.Dir(c.config.GeoDatabase)] = struct{}{}
	}
	if c.config.ASNDatabase != "" {
		dirs[filepath.Dir(c.config.ASNDatabase)] = struct{}{}
	}
	for k := range dirs {
		if err := watcher.Add(k); err != nil {
			c.r.Err(err).Msg("cannot watch database directory")
			return fmt.Errorf("cannot watch database directory: %w", err)
		}
	}
	c.t.Go(func() error {
		errLogger := c.r.Sample(reporter.BurstSampler(10*time.Second, 1))
		defer watcher.Close()

		for {
			// Watch both for errors and events in the
			// same goroutine. fsnotify's FAQ says this is
			// not a good idea.
			select {
			case <-c.t.Dying():
				return nil
			case err, ok := <-watcher.Errors:
				if !ok {
					return errors.New("file watcher died")
				}
				errLogger.Err(err).Msg("error from watcher")
			case event, ok := <-watcher.Events:
				if !ok {
					return errors.New("file watcher died")
				}
				if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
					continue
				}
				if filepath.Clean(event.Name) == c.config.GeoDatabase {
					c.openDatabase("geo", c.config.GeoDatabase, &c.db.geo)
				}
				if filepath.Clean(event.Name) == c.config.ASNDatabase {
					c.openDatabase("asn", c.config.ASNDatabase, &c.db.asn)
				}
			}
		}
	})
	return nil
}

// Stop stops the GeoIP component.
func (c *Component) Stop() error {
	if c.db.geo.Load() == nil && c.db.asn.Load() == nil {
		return nil
	}
	c.r.Info().Msg("stopping GeoIP component")
	defer c.r.Info().Msg("GeoIP component stopped")
	c.t.Kill(nil)
	return c.t.Wait()
}
