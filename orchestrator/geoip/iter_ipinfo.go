// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"strconv"

	"github.com/oschwald/maxminddb-golang"
)

type ipinfoDBASN struct {
	ASN    string `maxminddb:"asn"`
	ASName string `maxminddb:"as_name"`
}

type ipinfoDBCountry struct {
	Country string `maxminddb:"country"`
	Region  string `maxminddb:"region"`
	City    string `maxminddb:"city"`
}

type ipinfoDB struct {
	db *maxminddb.Reader
}

func (mmdb *ipinfoDB) IterASNDatabase(f AsnIterFunc) error {
	it := mmdb.db.Networks()
	maxminddb.SkipAliasedNetworks(it)
	for it.Next() {
		asnInfo := &ipinfoDBASN{}
		subnet, err := it.Network(asnInfo)

		if err != nil {
			return err
		}
		n, err := strconv.ParseUint(asnInfo.ASN[2:], 10, 32)
		if err != nil {
			return err
		}
		if err := f(subnet, ASNInfo{
			ASNumber: uint32(n),
			ASName:   asnInfo.ASName,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (mmdb *ipinfoDB) IterGeoDatabase(f GeoIterFunc) error {
	it := mmdb.db.Networks()
	maxminddb.SkipAliasedNetworks(it)
	for it.Next() {
		geoInfo := &ipinfoDBCountry{}
		subnet, err := it.Network(geoInfo)

		if err != nil {
			return err
		}
		if err := f(subnet, GeoInfo{
			Country: geoInfo.Country,
			State:   geoInfo.Region,
			City:    geoInfo.City,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (mmdb *ipinfoDB) Close() {
	mmdb.db.Close()
}
