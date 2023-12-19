// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package geoip provides ASN and country for GeoIP addresses.
package geoip

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
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
		geo  map[string]geoDatabase
		asn  map[string]geoDatabase
		lock sync.RWMutex
	}

	metrics struct {
		databaseRefresh *reporter.CounterVec
	}

	onOpenChan        chan DBNotification
	onOpenSubscribers []chan DBNotification
	notifyLock        sync.RWMutex
	notifyDone        sync.WaitGroup
}

// DBNotification is sent to all listener when a databased is opened/refreshed.
type DBNotification struct {
	Path  string
	Kind  string
	Index int
}

// Dependencies define the dependencies of the GeoIP component.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new GeoIP component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:                 r,
		d:                 &dependencies,
		config:            configuration,
		onOpenChan:        make(chan DBNotification),
		onOpenSubscribers: []chan DBNotification{},
	}
	c.db.geo = make(map[string]geoDatabase)
	c.db.asn = make(map[string]geoDatabase)

	for i, path := range c.config.GeoDatabase {
		c.config.GeoDatabase[i] = filepath.Clean(path)
	}
	for i, path := range c.config.ASNDatabase {
		c.config.ASNDatabase[i] = filepath.Clean(path)
	}
	c.d.Daemon.Track(&c.t, "orchestrator/geoip")
	c.metrics.databaseRefresh = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "db_refresh_total",
			Help: "Refresh event for a GeoIP database.",
		},
		[]string{"database"},
	)
	return &c, nil
}

func (c *Component) fanout(notif DBNotification) {
	c.notifyLock.RLock()
	defer c.notifyLock.RUnlock()
	for _, subChan := range c.onOpenSubscribers {
		select {
		case <-c.t.Dying():
			return
		case subChan <- notif:
		default:
		}
	}
}

// Start starts the GeoIP component.
func (c *Component) Start() error {
	if len(c.config.GeoDatabase) == 0 && len(c.config.ASNDatabase) == 0 {
		c.r.Warn().Msg("skipping GeoIP component: no database specified")
	}
	c.r.Info().Msg("starting GeoIP component")

	c.t.Go(func() error {
		// notifier fanout
		for notif := range c.onOpenChan {
			c.fanout(notif)
		}
		for _, c := range c.onOpenSubscribers {
			close(c)
		}
		return nil
	})

	for i, path := range c.config.GeoDatabase {
		if err := c.openDatabase("geo", i, path); err != nil && !c.config.Optional {
			return err
		}
	}
	for i, path := range c.config.ASNDatabase {
		if err := c.openDatabase("asn", i, path); err != nil && !c.config.Optional {
			return err
		}
	}

	// Watch for modifications
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		c.r.Err(err).Msg("cannot setup watcher for GeoIP databases")
		return fmt.Errorf("cannot setup watcher: %w", err)
	}
	dirs := map[string]struct{}{}
	for _, path := range c.config.GeoDatabase {
		dirs[filepath.Dir(path)] = struct{}{}
	}
	for _, path := range c.config.ASNDatabase {
		dirs[filepath.Dir(path)] = struct{}{}
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
				for i, path := range c.config.GeoDatabase {
					if filepath.Clean(event.Name) == path {
						c.openDatabase("geo", i, path)
						break
					}
				}
				for i, path := range c.config.ASNDatabase {
					if filepath.Clean(event.Name) == path {
						c.openDatabase("geo", i, path)
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
	c.db.lock.RLock()
	c.r.Debug().Msg("closing database files")

	for _, db := range c.db.geo {
		if db != nil {
			db.Close()
		}
	}
	for _, db := range c.db.asn {
		if db != nil {
			db.Close()
		}
	}
	c.db.lock.RUnlock()
	c.r.Debug().Msg("stopping child threads")
	c.t.Kill(nil)
	c.r.Debug().Msg("waiting for notification to be sent")
	c.notifyDone.Wait()
	close(c.onOpenChan)
	defer c.r.Info().Msg("GeoIP component stopped")
	return c.t.Wait()
}

// Notify is what parent component should call to get notified when a database is updated.
func (c *Component) Notify() (chan DBNotification, chan struct{}) {
	notifyChan := make(chan DBNotification)
	c.notifyLock.Lock()
	c.onOpenSubscribers = append(c.onOpenSubscribers, notifyChan)
	c.notifyLock.Unlock()
	initDoneChan := make(chan struct{})
	// send existing database when the client subscribes
	c.t.Go(func() error {
		c.db.lock.RLock()
		defer c.db.lock.RUnlock()
		for i, path := range c.config.GeoDatabase {
			// not loaded yet
			if _, has := c.db.geo[path]; !has {
				continue
			}
			// prevent the fanout thread from closing the channel until everying is written
			c.notifyDone.Add(1)
			defer c.notifyDone.Done()
			select {
			case <-c.t.Dying():
				return nil
			case notifyChan <- DBNotification{
				Path:  path,
				Kind:  "geo",
				Index: i,
			}:
				continue
			}
		}
		for i, path := range c.config.ASNDatabase {
			// not loaded yet
			if _, has := c.db.asn[path]; !has {
				continue
			}
			// prevent the fanout thread from closing the channel until everying is written
			c.notifyDone.Add(1)
			defer c.notifyDone.Done()
			select {
			case <-c.t.Dying():
				return nil
			case notifyChan <- DBNotification{
				Path:  path,
				Kind:  "asn",
				Index: i,
			}:
				continue
			}
		}
		close(initDoneChan)
		return nil
	})
	return notifyChan, initDoneChan
}
