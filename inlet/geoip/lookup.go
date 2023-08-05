// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"net"
	"net/netip"
)

// LookupASN returns the result of a lookup for an AS number.
func (c *Component) LookupASN(ip netip.Addr) uint32 {
	asnDB := c.db.asn.Load()
	if asnDB != nil {
		ip := ip.As16()
		asn, err := (*asnDB).LookupASN(net.IP(ip[:]))
		if err == nil && asn != 0 {
			c.metrics.databaseHit.WithLabelValues("asn").Inc()
			return asn
		}
		c.metrics.databaseMiss.WithLabelValues("asn").Inc()
	}
	return 0
}

// LookupCountry returns the result of a lookup for country.
func (c *Component) LookupCountry(ip netip.Addr) string {
	geoDB := c.db.geo.Load()
	if geoDB != nil {
		ip := ip.As16()
		country, err := (*geoDB).LookupCountry(net.IP(ip[:]))
		if err == nil && country != "" {
			c.metrics.databaseHit.WithLabelValues("geo").Inc()
			return country
		}
		c.metrics.databaseMiss.WithLabelValues("geo").Inc()
	}
	return ""
}
