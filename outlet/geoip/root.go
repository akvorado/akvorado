// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package geoip provides ASN and country for GeoIP addresses.
package geoip

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
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

	// databases is replaced on each refresh. Lookups only load it.
	databases atomic.Pointer[databases]
	// openLock serializes the refreshes.
	openLock sync.Mutex

	metrics struct {
		databaseRefresh *reporter.CounterVec
	}
}

// databases are the opened databases, in configuration order. An entry is nil
// when the database could not be opened. Once published, it is never modified.
type databases struct {
	geo []geoDatabase
	asn []geoDatabase
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
	c.config.GeoDatabase = cleanPaths(c.config.GeoDatabase)
	c.config.ASNDatabase = cleanPaths(c.config.ASNDatabase)
	c.databases.Store(&databases{
		geo: make([]geoDatabase, len(c.config.GeoDatabase)),
		asn: make([]geoDatabase, len(c.config.ASNDatabase)),
	})
	c.d.Daemon.Track(&c.t, "outlet/geoip")
	c.metrics.databaseRefresh = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "db_refresh_total",
			Help: "Refresh event for a GeoIP database.",
		},
		[]string{"database"},
	)
	return &c, nil
}

// cleanPaths normalizes the provided paths and removes the duplicates. When a
// path is repeated, only the last occurrence is kept, as later databases take
// precedence over the earlier ones.
func cleanPaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)
		result = slices.DeleteFunc(result, func(p string) bool { return p == path })
		result = append(result, path)
	}
	return result
}

// Start starts the GeoIP component.
func (c *Component) Start() error {
	// Ensure we have at least one goroutine, otherwise Stop() waits forever.
	c.t.Go(func() error {
		<-c.t.Dying()
		return nil
	})

	if len(c.config.GeoDatabase) == 0 && len(c.config.ASNDatabase) == 0 {
		c.r.Warn().Msg("skipping GeoIP component: no database specified")
		return nil
	}
	c.r.Info().Msg("starting GeoIP component")

	for _, path := range c.config.GeoDatabase {
		if err := c.openDatabase("geo", path); err != nil && !c.config.Optional {
			return err
		}
	}
	for _, path := range c.config.ASNDatabase {
		if err := c.openDatabase("asn", path); err != nil && !c.config.Optional {
			return err
		}
	}

	// Watch for modifications
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		c.r.Err(err).Msg("cannot setup watcher for GeoIP databases")
		return fmt.Errorf("cannot setup watcher: %w", err)
	}
	dirs := map[string]bool{}
	for _, path := range c.config.GeoDatabase {
		dirs[filepath.Dir(path)] = true
	}
	for _, path := range c.config.ASNDatabase {
		dirs[filepath.Dir(path)] = true
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
				c.r.Debug().Msgf("event %s on file %s", event, event.Name)
				for _, path := range c.config.GeoDatabase {
					if filepath.Clean(event.Name) == path {
						c.openDatabase("geo", path)
						break
					}
				}
				for _, path := range c.config.ASNDatabase {
					if filepath.Clean(event.Name) == path {
						c.openDatabase("asn", path)
						break
					}
				}
			}
		}
	})

	return nil
}

// Stop stops the GeoIP component.
func (c *Component) Stop() error {
	c.r.Info().Msg("stopping GeoIP component")
	c.r.Debug().Msg("stopping child threads")
	c.t.Kill(nil)
	err := c.t.Wait()

	// Only close the databases once nothing can look them up anymore.
	c.r.Debug().Msg("closing database files")
	current := c.databases.Load()
	for _, db := range slices.Concat(current.geo, current.asn) {
		if db != nil {
			db.Close()
		}
	}
	defer c.r.Info().Msg("GeoIP component stopped")
	return err
}
