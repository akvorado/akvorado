// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"fmt"
	"net"
	"strings"
	"sync/atomic"

	"github.com/oschwald/maxminddb-golang"
)

type geoDatabase interface {
	Close()
	LookupCountry(ip net.IP) (string, error)
	LookupASN(ip net.IP) (uint32, error)
}

// openDatabase opens the provided database and closes the current
// one. Do nothing if the path is empty.
func (c *Component) openDatabase(which string, path string, container *atomic.Pointer[geoDatabase]) error {
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
	oldOne := container.Swap(&newOne)
	c.metrics.databaseRefresh.WithLabelValues(which).Inc()
	if oldOne != nil {
		c.r.Debug().
			Str("database", path).
			Msgf("closing previous %s database", which)
		(*oldOne).Close()
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
