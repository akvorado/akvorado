// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"github.com/oschwald/maxminddb-golang/v2"
	"github.com/oschwald/maxminddb-golang/v2/mmdbdata"
)

// for a list fields available, see: https://github.com/oschwald/geoip2-golang/blob/main/reader.go

// maxmindGeoInfo is an alias for GeoInfo with MaxMind-specific unmarshaling
type maxmindGeoInfo GeoInfo

// UnmarshalMaxMindDBCursor implements custom unmarshaling for MaxMind geo format
func (g *maxmindGeoInfo) UnmarshalMaxMindDBCursor(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
	entries, err := cursor.MapReader()
	if err != nil {
		return mmdbdata.Cursor{}, mmdbdata.NormalizeUnmarshalError[maxmindGeoInfo](err)
	}
	next := entries.First()
	for range entries.Len() {
		key, value, keyErr := next.ReadMapKey()
		if keyErr != nil {
			return mmdbdata.Cursor{}, keyErr
		}
		switch string(key) {
		case "country":
			next, err = g.unmarshalCountry(value)
		case "city":
			next, err = g.unmarshalCity(value)
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

// unmarshalCountry keeps the ISO code of the country.
func (g *maxmindGeoInfo) unmarshalCountry(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
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
		if string(key) == "iso_code" {
			g.Country, next, err = value.ReadString()
		} else {
			next, err = value.Skip()
		}
		if err != nil {
			return mmdbdata.Cursor{}, err
		}
	}
	return entries.End(next)
}

// unmarshalCity looks for the localized names of the city.
func (g *maxmindGeoInfo) unmarshalCity(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
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
			next, err = g.unmarshalCityNames(value)
		} else {
			next, err = value.Skip()
		}
		if err != nil {
			return mmdbdata.Cursor{}, err
		}
	}
	return entries.End(next)
}

// unmarshalCityNames keeps the English name of the city.
func (g *maxmindGeoInfo) unmarshalCityNames(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
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
			g.City, next, err = value.ReadString()
		} else {
			next, err = value.Skip()
		}
		if err != nil {
			return mmdbdata.Cursor{}, err
		}
	}
	return entries.End(next)
}

// unmarshalSubdivisions keeps the ISO code of the first subdivision, which is
// the largest one. The remaining ones are skipped.
func (g *maxmindGeoInfo) unmarshalSubdivisions(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
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

// unmarshalSubdivision keeps the state of one subdivision. MaxMind exposes an
// iso_code; DB-IP, which uses the same GeoIP2 format, only exposes names.en.
// Prefer the ISO code and fall back to the English name.
func (g *maxmindGeoInfo) unmarshalSubdivision(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
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
		if string(key) == "iso_code" {
			g.State, next, err = value.ReadString()
		} else if string(key) == "names" && g.State == "" {
			// DB-IP has no iso_code for subdivisions, only names.en: use it as a
			// fallback (an iso_code, in any order, still takes precedence).
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

// unmarshalSubdivisionNames keeps the English name of the subdivision as the
// state. DB-IP has no iso_code for subdivisions, only names.en.
func (g *maxmindGeoInfo) unmarshalSubdivisionNames(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
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

// maxmindASNInfo is an alias for ASNInfo with MaxMind-specific unmarshaling
type maxmindASNInfo ASNInfo

// UnmarshalMaxMindDBCursor implements custom unmarshaling for MaxMind ASN format
func (a *maxmindASNInfo) UnmarshalMaxMindDBCursor(cursor mmdbdata.Cursor) (mmdbdata.Cursor, error) {
	entries, err := cursor.MapReader()
	if err != nil {
		return mmdbdata.Cursor{}, mmdbdata.NormalizeUnmarshalError[maxmindASNInfo](err)
	}
	next := entries.First()
	for range entries.Len() {
		key, value, keyErr := next.ReadMapKey()
		if keyErr != nil {
			return mmdbdata.Cursor{}, keyErr
		}
		if string(key) == "autonomous_system_number" {
			var asn uint64
			asn, next, err = value.ReadUint()
			if err == nil {
				if uint64(uint32(asn)) != asn {
					return mmdbdata.Cursor{}, mmdbdata.NewUnmarshalTypeError[uint32](asn)
				}
				a.ASNumber = uint32(asn)
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

type maxmindDB struct {
	db *maxminddb.Reader
}

func (mmdb *maxmindDB) IterGeoDatabase(f GeoIterFunc) {
	for result := range mmdb.db.Networks() {
		var info maxmindGeoInfo
		if err := result.Decode(&info); err != nil || info == (maxmindGeoInfo{}) {
			continue
		}
		f(result.Prefix(), GeoInfo(info))
	}
}

func (mmdb *maxmindDB) IterASNDatabase(f ASNIterFunc) {
	for result := range mmdb.db.Networks() {
		var info maxmindASNInfo
		if err := result.Decode(&info); err != nil || info.ASNumber == 0 {
			continue
		}
		f(result.Prefix(), ASNInfo(info))
	}
}

func (mmdb *maxmindDB) Close() {
	mmdb.db.Close()
}
