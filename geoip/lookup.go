package geoip

import (
	"net"

	"github.com/oschwald/geoip2-golang"
)

// LookupResult represents the result of a lookup
type LookupResult struct {
	ASN          uint
	Organization string
	Country      string
}

// Lookup returns the result of a lookup in the opened database.
func (c *Component) Lookup(ip net.IP) (result LookupResult) {
	countryDB := c.db.country.Load()
	if countryDB != nil {
		country, err := countryDB.(*geoip2.Reader).Country(ip)
		if err == nil {
			result.Country = country.Country.IsoCode
		}
	}
	asnDB := c.db.asn.Load()
	if asnDB != nil {
		asn, err := asnDB.(*geoip2.Reader).ASN(ip)
		if err == nil {
			result.ASN = asn.AutonomousSystemNumber
			result.Organization = asn.AutonomousSystemOrganization
		}
	}
	return
}
