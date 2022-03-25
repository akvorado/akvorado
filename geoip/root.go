// Package geoip provides ASN and country for GeoIP addresses.
package geoip

import (
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/oschwald/geoip2-golang"
	"gopkg.in/tomb.v2"

	"akvorado/daemon"
	"akvorado/reporter"
)

// Component represents the GeoIP component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	db struct {
		country atomic.Value // *geoip2.Reader
		asn     atomic.Value // *geoip2.Reader
	}
	metrics struct {
		databaseRefresh *reporter.CounterVec
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
	if c.config.CountryDatabase != "" {
		c.config.CountryDatabase = filepath.Clean(c.config.CountryDatabase)
	}
	if c.config.ASNDatabase != "" {
		c.config.ASNDatabase = filepath.Clean(c.config.ASNDatabase)
	}
	c.d.Daemon.Track(&c.t, "geoip")
	c.metrics.databaseRefresh = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "db_refresh_total",
			Help: "Refresh event for a GeoIP database.",
		},
		[]string{"database"},
	)
	return &c, nil
}

// openDatabase opens the provided database and closes the current
// one. Do nothing if the path is empty.
func (c *Component) openDatabase(which string, path string, container *atomic.Value) error {
	if path == "" {
		return nil
	}
	c.r.Debug().Str("database", path).Msgf("opening %s database", which)
	db, err := geoip2.Open(path)
	if err != nil {
		c.r.Err(err).
			Str("database", path).
			Msgf("cannot open %s database", which)
		return fmt.Errorf("cannot open %s database: %w", which, err)
	}
	old := container.Swap(db)
	c.metrics.databaseRefresh.WithLabelValues(which).Inc()
	if old != nil {
		c.r.Debug().
			Str("database", path).
			Msgf("closing previous %s database", which)
		old.(*geoip2.Reader).Close()
	}
	return nil
}

// Start starts the GeoIP component.
func (c *Component) Start() error {
	if err := c.openDatabase("country", c.config.CountryDatabase, &c.db.country); err != nil {
		return err
	}
	if err := c.openDatabase("asn", c.config.ASNDatabase, &c.db.asn); err != nil {
		return err
	}
	if c.db.country.Load() == nil && c.db.asn.Load() == nil {
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
	dirs := map[string]bool{}
	if c.config.CountryDatabase != "" {
		dirs[filepath.Dir(c.config.CountryDatabase)] = true
	}
	if c.config.ASNDatabase != "" {
		dirs[filepath.Dir(c.config.ASNDatabase)] = true
	}
	for k := range dirs {
		if err := watcher.Add(k); err != nil {
			c.r.Err(err).Msg("cannot watch database directory")
			return fmt.Errorf("cannot watch database directory: %w", err)
		}
	}
	c.t.Go(func() error {
		errLogger := c.r.Sample(reporter.BurstSampler(10*time.Second, 1))
		defer func() {
			watcher.Close()
		}()

		for {
			// Watch both for errors and events in the
			// same goroutine. fsnotify's FAQ says this is
			// not a good idea.
			select {
			case <-c.t.Dying():
				return nil
			case err := <-watcher.Errors:
				errLogger.Err(err).Msg("error from watcher")
			case event := <-watcher.Events:
				if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
					continue
				}
				if filepath.Clean(event.Name) == c.config.CountryDatabase {
					c.openDatabase("country", c.config.CountryDatabase, &c.db.country)
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
	if c.db.country.Load() == nil && c.db.asn.Load() == nil {
		return nil
	}
	c.r.Info().Msg("stopping GeoIP component")
	defer c.r.Info().Msg("GeoIP component stopped")
	c.t.Kill(nil)
	return c.t.Wait()
}
