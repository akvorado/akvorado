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

func (mmdb *ipinfoDB) IterGeoDatabase(f GeoIterFunc) {
	for result := range mmdb.db.Networks() {
		var info ipinfoGeoInfo
		if err := result.Decode(&info); err != nil || info == (ipinfoGeoInfo{}) {
			continue
		}
		f(result.Prefix(), GeoInfo(info))
	}
}

func (mmdb *ipinfoDB) IterASNDatabase(f ASNIterFunc) {
	for result := range mmdb.db.Networks() {
		var info ipinfoASNInfo
		if err := result.Decode(&info); err != nil || info.ASNumber == 0 {
			continue
		}
		f(result.Prefix(), ASNInfo(info))
	}
}

func (mmdb *ipinfoDB) Close() {
	mmdb.db.Close()
}
