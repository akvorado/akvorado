// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

// databaseCloseDelay is how long to wait before closing a database which was
// replaced by a newer version. This is a bit fragile, but when we were doing
// lookups in inlet, we closed the old database immediately and never had an
// issue. 10 seconds is plenty of time. The alternative requires
// synchronization.
const databaseCloseDelay = 10 * time.Second

type geoDatabase interface {
	Close()
	LookupGeo(netip.Addr) GeoInfo
	LookupASN(netip.Addr) ASNInfo
}

// openDatabase opens the provided database and closes the current
// one. Do nothing if the path is empty.
func (c *Component) openDatabase(which, path string) error {
	if path == "" {
		return nil
	}
	c.r.Debug().Str("database", path).Msgf("opening %s database", which)
	db, err := maxminddb.Open(path)
	if err != nil {
		c.r.Err(err).
			Str("database", path).
			Msgf("cannot open %s database", which)
		return fmt.Errorf("cannot open %s database: %w", which, err)
	}
	newOne, err := getGeoDatabase(db)
	if err != nil {
		db.Close()
		return err
	}

	// Where the database goes is decided by its position in the configuration.
	var configured []string
	switch which {
	case "asn":
		configured = c.config.ASNDatabase
	case "geo":
		configured = c.config.GeoDatabase
	}
	index := slices.Index(configured, path)
	if index < 0 {
		newOne.Close()
		return fmt.Errorf("unknown %s database %q", which, path)
	}

	c.openLock.Lock()
	defer c.openLock.Unlock()
	current := c.databases.Load()
	next := databases{geo: slices.Clone(current.geo), asn: slices.Clone(current.asn)}
	var oldOne geoDatabase
	if which == "asn" {
		oldOne, next.asn[index] = next.asn[index], newOne
	} else {
		oldOne, next.geo[index] = next.geo[index], newOne
	}
	c.databases.Store(&next)
	c.metrics.databaseRefresh.WithLabelValues(which).Inc()
	if oldOne != nil {
		c.closeDatabaseLater(oldOne, which, path)
	}
	return nil
}

// closeDatabaseLater closes a database once no lookup can be using it anymore.
// Lookups do not take a lock and a database is memory-mapped: closing it while
// it is still in use crashes the process.
func (c *Component) closeDatabaseLater(db geoDatabase, which, path string) {
	c.t.Go(func() error {
		select {
		case <-c.t.Dying():
			// Lookups are stopped before this component.
		case <-time.After(databaseCloseDelay):
		}
		c.r.Debug().
			Str("database", path).
			Msgf("closing previous %s database", which)
		db.Close()
		return nil
	})
}

// getGeoDatabase guesses the database format and instantiate the right one.
func getGeoDatabase(db *maxminddb.Reader) (geoDatabase, error) {
	// We should looks at the fields, but instead we use metadata and default to
	// Maxmind.
	if strings.HasPrefix(db.Metadata.DatabaseType, "ipinfo ") {
		return &ipinfoDB{db: db}, nil
	}
	return &maxmindDB{db: db}, nil
}
