// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"github.com/oschwald/maxminddb-golang/v2"
	"github.com/oschwald/maxminddb-golang/v2/mmdbdata"
)

// DB-IP databases use the GeoIP2 schema, so country (country.iso_code) and city
// (city.names.en) are read exactly like MaxMind and reuse its helpers. The
// difference is the subdivision: DB-IP does not provide subdivisions[0].iso_code,
// only subdivisions[0].names.en (e.g. "California", "Saxony"). We therefore keep
// the English name in State, matching what ipinfoDB does with its region names.
// DB-IP ASN databases share the GeoLite2-ASN schema (autonomous_system_number),
// so they reuse maxmindASNInfo.

// dbipGeoInfo is an alias for GeoInfo with DB-IP-specific unmarshaling
type dbipGeoInfo GeoInfo

// UnmarshalMaxMindDBCursor implements custom unmarshaling for DB-IP geo format
func (g *dbipGeoInfo) UnmarshalMaxMindDBCursor(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
	entries, err := cursor.MapReader()
	if err != nil {
		return mmdbdata.Cursor{}, mmdbdata.NormalizeUnmarshalError[dbipGeoInfo](err)
	}
	next := entries.First()
	for range entries.Len() {
		key, value, keyErr := next.ReadMapKey()
		if keyErr != nil {
			return mmdbdata.Cursor{}, keyErr
		}
		switch string(key) {
		case "country":
			next, err = (*maxmindGeoInfo)(g).unmarshalCountry(value)
		case "city":
			next, err = (*maxmindGeoInfo)(g).unmarshalCity(value)
		case "subdivisions":
			next, err = g.unmarshalSubdivisions(value)
		default:
			next, err = value.Skip()
		}
		if err != nil {
			return mmdbdata.Cursor{}, err
		}
	}
	return entries.End(next)
}

// unmarshalSubdivisions keeps the English name of the first subdivision, which
// is the largest one. The remaining ones are skipped.
func (g *dbipGeoInfo) unmarshalSubdivisions(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
	values, err := cursor.Slice()
	if err != nil {
		return mmdbdata.Cursor{}, err
	}
	var next mmdbdata.Cursor
	for {
		index, value, ok := values.Next(next)
		if !ok {
			break
		}
		if index == 0 {
			next, err = g.unmarshalSubdivision(value)
		} else {
			next, err = value.Skip()
		}
		if err != nil {
			return mmdbdata.Cursor{}, err
		}
	}
	return values.End()
}

// unmarshalSubdivision looks for the localized names of one subdivision. DB-IP
// has no iso_code for subdivisions, only names.en.
func (g *dbipGeoInfo) unmarshalSubdivision(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
	entries, err := cursor.MapReader()
	if err != nil {
		return mmdbdata.Cursor{}, err
	}
	next := entries.First()
	for range entries.Len() {
		key, value, keyErr := next.ReadMapKey()
		if keyErr != nil {
			return mmdbdata.Cursor{}, keyErr
		}
		if string(key) == "names" {
			next, err = g.unmarshalSubdivisionNames(value)
		} else {
			next, err = value.Skip()
		}
		if err != nil {
			return mmdbdata.Cursor{}, err
		}
	}
	return entries.End(next)
}

// unmarshalSubdivisionNames keeps the English name of the subdivision.
func (g *dbipGeoInfo) unmarshalSubdivisionNames(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
	entries, err := cursor.MapReader()
	if err != nil {
		return mmdbdata.Cursor{}, err
	}
	next := entries.First()
	for range entries.Len() {
		key, value, keyErr := next.ReadMapKey()
		if keyErr != nil {
			return mmdbdata.Cursor{}, keyErr
		}
		if string(key) == "en" {
			g.State, next, err = value.ReadString()
		} else {
			next, err = value.Skip()
		}
		if err != nil {
			return mmdbdata.Cursor{}, err
		}
	}
	return entries.End(next)
}

type dbipDB struct {
	db *maxminddb.Reader
}

func (mmdb *dbipDB) IterGeoDatabase(f GeoIterFunc) {
	for result := range mmdb.db.Networks() {
		var info dbipGeoInfo
		if err := result.Decode(&info); err != nil || info == (dbipGeoInfo{}) {
			continue
		}
		f(result.Prefix(), GeoInfo(info))
	}
}

func (mmdb *dbipDB) IterASNDatabase(f ASNIterFunc) {
	for result := range mmdb.db.Networks() {
		var info maxmindASNInfo
		if err := result.Decode(&info); err != nil || info.ASNumber == 0 {
			continue
		}
		f(result.Prefix(), ASNInfo(info))
	}
}

func (mmdb *dbipDB) Close() {
	mmdb.db.Close()
}
