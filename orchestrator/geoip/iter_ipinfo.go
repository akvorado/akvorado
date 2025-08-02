// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"errors"
	"strconv"

	"github.com/oschwald/maxminddb-golang/v2"
	"github.com/oschwald/maxminddb-golang/v2/mmdbdata"
)

// ipinfoGeoInfo is an alias for GeoInfo with ipinfo-specific unmarshaling
type ipinfoGeoInfo GeoInfo

// UnmarshalMaxMindDB implements custom unmarshaling for ipinfo geo format
func (g *ipinfoGeoInfo) UnmarshalMaxMindDB(d *mmdbdata.Decoder) error {
	mapIter, _, err := d.ReadMap()
	if err != nil {
		return err
	}

	for key, err := range mapIter {
		if err != nil {
			return err
		}
		switch string(key) {
		case "country":
			g.Country, err = d.ReadString()
		case "region":
			g.State, err = d.ReadString()
		case "city":
			g.City, err = d.ReadString()
		default:
			err = d.SkipValue()
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// ipinfoASNInfo is an alias for ASNInfo with ipinfo-specific unmarshaling
type ipinfoASNInfo ASNInfo

// UnmarshalMaxMindDB implements custom unmarshaling for ipinfo ASN format
func (a *ipinfoASNInfo) UnmarshalMaxMindDB(d *mmdbdata.Decoder) error {
	mapIter, _, err := d.ReadMap()
	if err != nil {
		return err
	}

	for key, err := range mapIter {
		if err != nil {
			return err
		}
		switch string(key) {
		case "asn":
			asnStr, err := d.ReadString()
			// Parse ASN from "ASxxxx" format
			if err == nil && len(asnStr) > 2 && asnStr[:2] == "AS" {
				if num, err := strconv.ParseUint(asnStr[2:], 10, 32); err == nil {
					a.ASNumber = uint32(num)
					continue
				}
			}
			return errors.New("invalid AS number")
		default:
			if err := d.SkipValue(); err != nil {
				return err
			}
		}
	}
	return nil
}

type ipinfoDB struct {
	db *maxminddb.Reader
}

func (mmdb *ipinfoDB) IterASNDatabase(f AsnIterFunc) error {
	for result := range mmdb.db.Networks() {
		var asnInfo ipinfoASNInfo

		err := result.Decode(&asnInfo)
		if err != nil || asnInfo.ASNumber == 0 {
			continue
		}

		prefix := result.Prefix()
		if err := f(prefix, ASNInfo(asnInfo)); err != nil {
			return err
		}
	}
	return nil
}

func (mmdb *ipinfoDB) IterGeoDatabase(f GeoIterFunc) error {
	for result := range mmdb.db.Networks() {
		var geoInfo ipinfoGeoInfo

		err := result.Decode(&geoInfo)
		if err != nil || geoInfo.Country == "" {
			continue
		}

		prefix := result.Prefix()
		if err := f(prefix, GeoInfo(geoInfo)); err != nil {
			return err
		}
	}
	return nil
}

func (mmdb *ipinfoDB) Close() {
	mmdb.db.Close()
}
