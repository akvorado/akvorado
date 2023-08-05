// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"net"
	"strconv"

	"github.com/oschwald/maxminddb-golang"
)

type ipinfoDBASN struct {
	ASN string `maxminddb:"asn"`
}

type ipinfoDBCountry struct {
	Country string `maxminddb:"country"`
}

type ipinfoDB struct {
	db *maxminddb.Reader
}

// LookupASN returns the result of a lookup for an AS number.
func (mmdb *ipinfoDB) LookupASN(ip net.IP) (uint32, error) {
	var asn ipinfoDBASN
	if err := mmdb.db.Lookup(ip, &asn); err != nil {
		return 0, err
	}
	if asn.ASN == "" {
		return 0, nil
	}
	n, err := strconv.ParseUint(asn.ASN[2:], 10, 32)
	if err != nil {
		return 0, nil
	}
	return uint32(n), nil
}

// LookupCountry returns the result of a lookup for country.
func (mmdb *ipinfoDB) LookupCountry(ip net.IP) (string, error) {
	var country ipinfoDBCountry
	if err := mmdb.db.Lookup(ip, &country); err != nil {
		return "", err
	}
	return country.Country, nil
}

func (mmdb *ipinfoDB) Close() {
	mmdb.db.Close()
}
