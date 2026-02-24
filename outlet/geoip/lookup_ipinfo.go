// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"errors"
	"net/netip"
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

func (mmdb *ipinfoDB) LookupGeo(ip netip.Addr) GeoInfo {
	var info ipinfoGeoInfo
	_ = mmdb.db.Lookup(ip).Decode(&info)
	return GeoInfo(info)
}

func (mmdb *ipinfoDB) LookupASN(ip netip.Addr) ASNInfo {
	var info ipinfoASNInfo
	_ = mmdb.db.Lookup(ip).Decode(&info)
	return ASNInfo(info)
}

func (mmdb *ipinfoDB) Close() {
	mmdb.db.Close()
}
