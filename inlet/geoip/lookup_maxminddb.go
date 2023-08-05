// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"net"

	"github.com/oschwald/maxminddb-golang"
)

type maxmindDBASN struct {
	AutonomousSystemNumber uint `maxminddb:"autonomous_system_number"`
}

type maxmindDBCountry struct {
	Country struct {
		IsoCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

type maxmindDB struct {
	db *maxminddb.Reader
}

// LookupASN returns the result of a lookup for an AS number.
func (mmdb *maxmindDB) LookupASN(ip net.IP) (uint32, error) {
	var asn maxmindDBASN
	if err := mmdb.db.Lookup(ip, &asn); err != nil {
		return 0, err
	}
	return uint32(asn.AutonomousSystemNumber), nil
}

// LookupCountry returns the result of a lookup for country.
func (mmdb *maxmindDB) LookupCountry(ip net.IP) (string, error) {
	var country maxmindDBCountry
	if err := mmdb.db.Lookup(ip, &country); err != nil {
		return "", err
	}
	return country.Country.IsoCode, nil
}

func (mmdb *maxmindDB) Close() {
	mmdb.db.Close()
}
