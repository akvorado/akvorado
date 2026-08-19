// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"fmt"
	"slices"
	"strings"

	"github.com/oschwald/maxminddb-golang/v2"
)

type geoDatabase interface {
	Close()
	IterGeoDatabase(GeoIterFunc)
	IterASNDatabase(ASNIterFunc)
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
	c.notify()
	if oldOne != nil {
		// A database is memory-mapped: closing it while it is walked crashes the
		// process. New walks use the database we just stored, wait for the
		// current one to be over. A walk takes a long time and the caller is the
		// goroutine watching for file events: waiting here would make it miss
		// some of them, therefore this is done in the background.
		c.t.Go(func() error {
			c.iterLock.Lock()
			defer c.iterLock.Unlock()
			c.r.Debug().
				Str("database", path).
				Msgf("closing previous %s database", which)
			oldOne.Close()
			return nil
		})
	}
	return nil
}

// getGeoDatabase guesses the database format and instantiate the right one.
func getGeoDatabase(db *maxminddb.Reader) (geoDatabase, error) {
	// We should looks at the fields, but instead we use metadata and default to
	// Maxmind.
	if strings.HasPrefix(db.Metadata.DatabaseType, "ipinfo ") {
		return &ipinfoDB{db: db}, nil
	}
	// DB-IP databases use the GeoIP2 schema but expose subdivision names instead
	// of ISO codes. Their DatabaseType is e.g. "DBIP-City-Lite", "DBIP-Location".
	if strings.HasPrefix(db.Metadata.DatabaseType, "DBIP") {
		return &dbipDB{db: db}, nil
	}
	return &maxmindDB{db: db}, nil
}
