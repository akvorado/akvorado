// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"fmt"
)

// GeoInfo describes geographical data of a geo the database.
type GeoInfo struct {
	Country string
	City    string
	State   string
}

// ASNInfo describes ASN data of an ASN database.
type ASNInfo struct {
	ASNumber uint32
}

// IterGeoDatabases iter all entries in all geo databases.
func (c *Component) IterGeoDatabases(f GeoIterFunc) error {
	c.db.lock.RLock()
	defer c.db.lock.RUnlock()
	for _, path := range c.config.GeoDatabase {
		geoDB, ok := c.db.geo[path]
		if !ok && c.config.Optional {
			continue
		} else if !ok {
			return fmt.Errorf("database not found %s", path)
		}
		if err := geoDB.IterGeoDatabase(f); err != nil {
			return err
		}
	}
	return nil
}

// IterASNDatabases iter all entries in all ASN databases.
func (c *Component) IterASNDatabases(f AsnIterFunc) error {
	c.db.lock.RLock()
	defer c.db.lock.RUnlock()
	for _, path := range c.config.ASNDatabase {
		asnDB, ok := c.db.asn[path]
		if !ok && c.config.Optional {
			continue
		} else if !ok {
			return fmt.Errorf("database not found %s", path)
		}
		if err := asnDB.IterASNDatabase(f); err != nil {
			return err
		}
	}
	return nil
}
