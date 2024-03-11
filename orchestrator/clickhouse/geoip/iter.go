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

// ASNInfo describes ASN data of an asn database.
type ASNInfo struct {
	ASNumber uint32
	ASName   string
}

// IterGeoDatabase iter all entries in the given geo database path.
func (c *Component) IterGeoDatabase(path string, f GeoIterFunc) error {
	c.db.lock.RLock()
	defer c.db.lock.RUnlock()
	geoDB := c.db.geo[path]
	if geoDB != nil {
		return geoDB.IterGeoDatabase(f)
	}

	return fmt.Errorf("database not found %s", path)
}

// IterASNDatabase iter all entries in the given asn database path.
func (c *Component) IterASNDatabase(path string, f AsnIterFunc) error {
	c.db.lock.RLock()
	defer c.db.lock.RUnlock()
	geoDB := c.db.asn[path]
	if geoDB != nil {
		return geoDB.IterASNDatabase(f)
	}

	return fmt.Errorf("database not found %s", path)
}
