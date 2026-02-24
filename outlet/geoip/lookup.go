// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"net/netip"
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

// LookupGeo performs a GeoIP geo lookup across all configured geo databases.
// Databases are iterated in config order; last non-empty field wins.
func (c *Component) LookupGeo(ip netip.Addr) GeoInfo {
	ip = ip.Unmap()
	var result GeoInfo
	for _, geoDB := range c.databases.Load().geo {
		if geoDB == nil {
			continue
		}
		info := geoDB.LookupGeo(ip)
		if info.Country != "" {
			result.Country = info.Country
		}
		if info.State != "" {
			result.State = info.State
		}
		if info.City != "" {
			result.City = info.City
		}
	}
	return result
}

// LookupASN performs a GeoIP ASN lookup across all configured ASN databases.
// Databases are iterated in config order; last non-zero value wins.
func (c *Component) LookupASN(ip netip.Addr) ASNInfo {
	ip = ip.Unmap()
	var result ASNInfo
	for _, asnDB := range c.databases.Load().asn {
		if asnDB == nil {
			continue
		}
		info := asnDB.LookupASN(ip)
		if info.ASNumber != 0 {
			result.ASNumber = info.ASNumber
		}
	}
	return result
}
