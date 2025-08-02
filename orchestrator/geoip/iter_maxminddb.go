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

// UnmarshalMaxMindDB implements custom unmarshaling for MaxMind geo format
func (g *maxmindGeoInfo) UnmarshalMaxMindDB(d *mmdbdata.Decoder) error {
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
			countryIter, _, err := d.ReadMap()
			if err != nil {
				return err
			}
			for countryKey, err := range countryIter {
				if err != nil {
					return err
				}
				if string(countryKey) == "iso_code" {
					isoCode, err := d.ReadString()
					if err != nil {
						return err
					}
					g.Country = isoCode
				} else {
					if err := d.SkipValue(); err != nil {
						return err
					}
				}
			}
		case "city":
			cityIter, _, err := d.ReadMap()
			if err != nil {
				return err
			}
			for cityKey, err := range cityIter {
				if err != nil {
					return err
				}
				if string(cityKey) == "names" {
					namesIter, _, err := d.ReadMap()
					if err != nil {
						return err
					}
					for nameKey, err := range namesIter {
						if err != nil {
							return err
						}
						if string(nameKey) == "en" {
							cityName, err := d.ReadString()
							if err != nil {
								return err
							}
							g.City = cityName
						} else {
							if err := d.SkipValue(); err != nil {
								return err
							}
						}
					}
				} else {
					if err := d.SkipValue(); err != nil {
						return err
					}
				}
			}
		case "subdivisions":
			subdivisionsIter, _, err := d.ReadSlice()
			if err != nil {
				return err
			}
			skipRemaining := false
			for err := range subdivisionsIter {
				if err != nil {
					return err
				}
				if !skipRemaining {
					subdivisionIter, _, err := d.ReadMap()
					if err != nil {
						return err
					}
					for subdivisionKey, err := range subdivisionIter {
						if err != nil {
							return err
						}
						if string(subdivisionKey) == "iso_code" {
							isoCode, err := d.ReadString()
							if err != nil {
								return err
							}
							g.State = isoCode
						} else {
							if err := d.SkipValue(); err != nil {
								return err
							}
						}
					}
					skipRemaining = true
				} else {
					if err := d.SkipValue(); err != nil {
						return err
					}
				}
			}
		default:
			if err := d.SkipValue(); err != nil {
				return err
			}
		}
	}
	return nil
}

// maxmindASNInfo is an alias for ASNInfo with MaxMind-specific unmarshaling
type maxmindASNInfo ASNInfo

// UnmarshalMaxMindDB implements custom unmarshaling for MaxMind ASN format
func (a *maxmindASNInfo) UnmarshalMaxMindDB(d *mmdbdata.Decoder) error {
	mapIter, _, err := d.ReadMap()
	if err != nil {
		return err
	}

	for key, err := range mapIter {
		if err != nil {
			return err
		}
		switch string(key) {
		case "autonomous_system_number":
			asn, err := d.ReadUint32()
			if err != nil {
				return err
			}
			a.ASNumber = asn
		default:
			if err := d.SkipValue(); err != nil {
				return err
			}
		}
	}
	return nil
}

type maxmindDB struct {
	db *maxminddb.Reader
}

func (mmdb *maxmindDB) IterASNDatabase(f AsnIterFunc) error {
	for result := range mmdb.db.Networks() {
		var asnInfo maxmindASNInfo

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

func (mmdb *maxmindDB) IterGeoDatabase(f GeoIterFunc) error {
	for result := range mmdb.db.Networks() {
		var geoInfo maxmindGeoInfo

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

func (mmdb *maxmindDB) Close() {
	mmdb.db.Close()
}
