// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"github.com/oschwald/maxminddb-golang/v2"
)

// for a list fields available, see: https://github.com/oschwald/geoip2-golang/blob/main/reader.go

type maxmindDB struct {
	db *maxminddb.Reader
}

func (mmdb *maxmindDB) IterASNDatabase(f AsnIterFunc) error {
	for result := range mmdb.db.Networks() {
		var asn uint32

		// Get AS number, skip if not found
		result.DecodePath(&asn, "autonomous_system_number")
		if asn == 0 {
			continue
		}
		prefix := result.Prefix()
		if err := f(prefix, ASNInfo{
			ASNumber: asn,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (mmdb *maxmindDB) IterGeoDatabase(f GeoIterFunc) error {
	for result := range mmdb.db.Networks() {
		var country string
		var city string
		var state string

		// Get country, city, and state. Skip if no country
		result.DecodePath(&country, "country", "iso_code")
		if country == "" {
			continue
		}
		result.DecodePath(&city, "city", "names", "en")
		result.DecodePath(&state, "subdivisions", "0", "iso_code")

		prefix := result.Prefix()
		if err := f(prefix, GeoInfo{
			Country: country,
			State:   state,
			City:    city,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (mmdb *maxmindDB) Close() {
	mmdb.db.Close()
}
