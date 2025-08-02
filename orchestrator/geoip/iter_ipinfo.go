// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"strconv"

	"github.com/oschwald/maxminddb-golang/v2"
)

type ipinfoDB struct {
	db *maxminddb.Reader
}

func (mmdb *ipinfoDB) IterASNDatabase(f AsnIterFunc) error {
	for result := range mmdb.db.Networks() {
		var asnStr string // They are stored as ASxxxx

		// Get AS number, skip if not found
		result.DecodePath(&asnStr, "asn")
		if asnStr == "" {
			continue
		}
		asn, err := strconv.ParseUint(asnStr[2:], 10, 32)
		if err != nil {
			continue
		}

		prefix := result.Prefix()
		if err := f(prefix, ASNInfo{
			ASNumber: uint32(asn),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (mmdb *ipinfoDB) IterGeoDatabase(f GeoIterFunc) error {
	for result := range mmdb.db.Networks() {
		var country, region, city string

		// Get country, region, and city
		result.DecodePath(&country, "country")
		result.DecodePath(&region, "region")
		result.DecodePath(&city, "city")

		prefix := result.Prefix()
		if err := f(prefix, GeoInfo{
			Country: country,
			State:   region,
			City:    city,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (mmdb *ipinfoDB) Close() {
	mmdb.db.Close()
}
