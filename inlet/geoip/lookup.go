// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"net"
	"net/netip"
)

type asn struct {
	AutonomousSystemNumber uint `maxminddb:"autonomous_system_number"`
}

type country struct {
	Country struct {
		IsoCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

// LookupASN returns the result of a lookup for an AS number.
func (c *Component) LookupASN(ip netip.Addr) uint32 {
	asnDB := c.db.asn.Load()
	if asnDB != nil {
		var asn asn
		ip := ip.As16()
		err := asnDB.Lookup(net.IP(ip[:]), &asn)
		if err == nil && asn.AutonomousSystemNumber != 0 {
			c.metrics.databaseHit.WithLabelValues("asn").Inc()
			return uint32(asn.AutonomousSystemNumber)
		}
		c.metrics.databaseMiss.WithLabelValues("asn").Inc()
	}
	return 0
}

// LookupCountry returns the result of a lookup for country.
func (c *Component) LookupCountry(ip netip.Addr) string {
	geoDB := c.db.geo.Load()
	if geoDB != nil {
		var country country
		ip := ip.As16()
		err := geoDB.Lookup(net.IP(ip[:]), &country)
		if err == nil && country.Country.IsoCode != "" {
			c.metrics.databaseHit.WithLabelValues("geo").Inc()
			return country.Country.IsoCode
		}
		c.metrics.databaseMiss.WithLabelValues("geo").Inc()
	}
	return ""
}
