// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"fmt"
	"net"
	"strings"

	"github.com/oschwald/maxminddb-golang"
)

// GeoIterFunc is the required signature to iter a geo database.
type GeoIterFunc func(*net.IPNet, GeoInfo) error

// AsnIterFunc is the required signature to iter an asn database;
type AsnIterFunc func(*net.IPNet, ASNInfo) error

type geoDatabase interface {
	Close()
	IterASNDatabase(AsnIterFunc) error
	IterGeoDatabase(GeoIterFunc) error
}

// openDatabase opens the provided database and closes the current
// one. Do nothing if the path is empty.
func (c *Component) openDatabase(which string, path string, notifySubscribers bool) error {
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
		return err
	}
	c.db.lock.Lock()
	defer c.db.lock.Unlock()
	var oldOne geoDatabase
	switch which {
	case "asn":
		oldOne = c.db.asn[path]
		c.db.asn[path] = newOne
	case "geo":
		oldOne = c.db.geo[path]
		c.db.geo[path] = newOne
	}
	c.metrics.databaseRefresh.WithLabelValues(which).Inc()
	if oldOne != nil {
		c.r.Debug().
			Str("database", path).
			Msgf("closing previous %s database", which)
		oldOne.Close()
	}
	if notifySubscribers {
		c.notifyDone.Add(1)
		c.onOpenChan <- struct{}{}
		c.notifyDone.Done()
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
	return &maxmindDB{db: db}, nil
}
