// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"github.com/oschwald/maxminddb-golang"
)

type maxmindDBASN struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

// for a list fields available, see: https://github.com/oschwald/geoip2-golang/blob/main/reader.go
type maxmindDBCountry struct {
	Country struct {
		IsoCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Subdivisions []struct {
		IsoCode string `maxminddb:"iso_code"`
	} `maxminddb:"subdivisions"`
}

type maxmindDB struct {
	db *maxminddb.Reader
}

func (mmdb *maxmindDB) IterASNDatabase(f AsnIterFunc) error {
	it := mmdb.db.Networks()
	maxminddb.SkipAliasedNetworks(it)

	for it.Next() {
		asnInfo := &maxmindDBASN{}
		subnet, err := it.Network(asnInfo)

		if err != nil {
			return err
		}
		if err := f(subnet, ASNInfo{
			ASNumber: uint32(asnInfo.AutonomousSystemNumber),
			ASName:   asnInfo.AutonomousSystemOrganization,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (mmdb *maxmindDB) IterGeoDatabase(f GeoIterFunc) error {
	it := mmdb.db.Networks()
	maxminddb.SkipAliasedNetworks(it)

	for it.Next() {
		geoInfo := &maxmindDBCountry{}
		subnet, err := it.Network(geoInfo)

		if err != nil {
			return err
		}
		var state string
		if len(geoInfo.Subdivisions) > 0 {
			state = geoInfo.Subdivisions[0].IsoCode
		}

		if err := f(subnet, GeoInfo{
			Country: geoInfo.Country.IsoCode,
			State:   state,
			City:    geoInfo.City.Names["en"],
		}); err != nil {
			return err
		}
	}
	return nil
}

func (mmdb *maxmindDB) Close() {
	mmdb.db.Close()
}
