// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"net"

	"github.com/oschwald/geoip2-golang"
)

// LookupASN returns the result of a lookup for an AS number.
func (c *Component) LookupASN(ip net.IP) uint32 {
	asnDB := c.db.asn.Load()
	if asnDB != nil {
		asn, err := asnDB.(*geoip2.Reader).ASN(ip)
		if err == nil && asn.AutonomousSystemNumber != 0 {
			c.metrics.databaseHit.WithLabelValues("asn").Inc()
			return uint32(asn.AutonomousSystemNumber)
		}
		c.metrics.databaseMiss.WithLabelValues("asn").Inc()
	}
	return 0
}

// LookupCountry returns the result of a lookup for country.
func (c *Component) LookupCountry(ip net.IP) string {
	geoDB := c.db.geo.Load()
	if geoDB != nil {
		geo, err := geoDB.(*geoip2.Reader).Country(ip)
		if err == nil && geo.Country.IsoCode != "" {
			c.metrics.databaseHit.WithLabelValues("geo").Inc()
			return geo.Country.IsoCode
		}
		c.metrics.databaseMiss.WithLabelValues("geo").Inc()
	}
	return ""
}
