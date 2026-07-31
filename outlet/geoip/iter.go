// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"net/netip"
)

// GeoInfo describes geographical data of a geo database.
type GeoInfo struct {
	Country string
	City    string
	State   string
}

// ASNInfo describes ASN data of an ASN database.
type ASNInfo struct {
	ASNumber uint32
}

// GeoIterFunc is the required signature to iterate over a geo database.
type GeoIterFunc func(netip.Prefix, GeoInfo)

// ASNIterFunc is the required signature to iterate over an ASN database.
type ASNIterFunc func(netip.Prefix, ASNInfo)

// IterGeoDatabases iterates over all the entries of all the geo databases, in
// configuration order. Databases which could not be opened are skipped, as are
// the records which cannot be decoded: walking never fails.
func (c *Component) IterGeoDatabases(f GeoIterFunc) {
	c.iterLock.Lock()
	defer c.iterLock.Unlock()
	for _, geoDB := range c.databases.Load().geo {
		if geoDB == nil {
			continue
		}
		geoDB.IterGeoDatabase(f)
	}
}

// IterASNDatabases iterates over all the entries of all the ASN databases, in
// configuration order. Databases which could not be opened are skipped, as are
// the records which cannot be decoded: walking never fails.
func (c *Component) IterASNDatabases(f ASNIterFunc) {
	c.iterLock.Lock()
	defer c.iterLock.Unlock()
	for _, asnDB := range c.databases.Load().asn {
		if asnDB == nil {
			continue
		}
		asnDB.IterASNDatabase(f)
	}
}
