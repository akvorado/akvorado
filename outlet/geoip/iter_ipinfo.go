// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"errors"
	"strconv"
	"strings"

	"github.com/oschwald/maxminddb-golang/v2"
	"github.com/oschwald/maxminddb-golang/v2/mmdbdata"
)

// ipinfoGeoInfo is an alias for GeoInfo with ipinfo-specific unmarshaling
type ipinfoGeoInfo GeoInfo

// UnmarshalMaxMindDBCursor implements custom unmarshaling for ipinfo geo format
func (g *ipinfoGeoInfo) UnmarshalMaxMindDBCursor(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
	entries, err := cursor.MapReader()
	if err != nil {
		return mmdbdata.Cursor{}, mmdbdata.NormalizeUnmarshalError[ipinfoGeoInfo](err)
	}
	next := entries.First()
	for range entries.Len() {
		key, value, keyErr := next.ReadMapKey()
		if keyErr != nil {
			return mmdbdata.Cursor{}, keyErr
		}
		switch string(key) {
		case "country":
			g.Country, next, err = value.ReadString()
		case "region":
			g.State, next, err = value.ReadString()
		case "city":
			g.City, next, err = value.ReadString()
		default:
			next, err = value.Skip()
		}
		if err != nil {
			return mmdbdata.Cursor{}, err
		}
	}
	return entries.End(next)
}

// ipinfoASNInfo is an alias for ASNInfo with ipinfo-specific unmarshaling
type ipinfoASNInfo ASNInfo

// UnmarshalMaxMindDBCursor implements custom unmarshaling for ipinfo ASN format
func (a *ipinfoASNInfo) UnmarshalMaxMindDBCursor(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
	entries, err := cursor.MapReader()
	if err != nil {
		return mmdbdata.Cursor{}, mmdbdata.NormalizeUnmarshalError[ipinfoASNInfo](err)
	}
	next := entries.First()
	for range entries.Len() {
		key, value, keyErr := next.ReadMapKey()
		if keyErr != nil {
			return mmdbdata.Cursor{}, keyErr
		}
		if string(key) == "asn" {
			// The AS number uses the "ASxxxx" format.
			var asn string
			asn, next, err = value.ReadString()
			if err == nil {
				digits, found := strings.CutPrefix(asn, "AS")
				num, parseErr := strconv.ParseUint(digits, 10, 32)
				if !found || parseErr != nil {
					return mmdbdata.Cursor{}, errors.New("invalid AS number")
				}
				a.ASNumber = uint32(num)
			}
		} else {
			next, err = value.Skip()
		}
		if err != nil {
			return mmdbdata.Cursor{}, err
		}
	}
	return entries.End(next)
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
