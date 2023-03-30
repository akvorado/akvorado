// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import "errors"

// Configuration describes the configuration for the schema component.
type Configuration struct {
	// Disabled lists the columns disabled (in addition to the ones disabled by default).
	Disabled []ColumnKey
	// Enabled lists the columns enabled (in addition to the ones enabled by default).
	Enabled []ColumnKey `validate:"ninterfield=Disabled"`
	// MainTableOnly lists columns to be moved to the main table only
	MainTableOnly []ColumnKey
	// NotMainTableOnly lists columns to be moved out of the main table only
	NotMainTableOnly []ColumnKey `validate:"ninterfield=MainTableOnly"`
	// Generate lists columns that shall be generated at ingest instead of generated at query time
	Generate []ColumnKey
}

// DefaultConfiguration returns the default configuration for the schema component.
func DefaultConfiguration() Configuration {
	return Configuration{}
}

// MarshalText turns a column key to text
func (ck ColumnKey) MarshalText() ([]byte, error) {
	got, ok := columnNameMap.LoadValue(ck)
	if ok {
		return []byte(got), nil
	}
	return nil, errors.New("unknown column name")
}

func (ck ColumnKey) String() string {
	name, _ := columnNameMap.LoadValue(ck)
	return name
}

// UnmarshalText provides a column key from text
func (ck *ColumnKey) UnmarshalText(input []byte) error {
	got, ok := columnNameMap.LoadKey(string(input))
	if ok {
		*ck = got
		return nil
	}
	return errors.New("unknown provider")
}
