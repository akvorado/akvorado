// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"errors"

	"akvorado/common/helpers"
	"akvorado/common/helpers/bimap"
)

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
	// Materialize lists columns that shall be materialized at ingest instead of computed at query time
	Materialize []ColumnKey
	// CustomDictionaries allows enrichment of flows with custom metadata
	CustomDictionaries map[string]CustomDict `validate:"dive"`
}

// CustomDict represents a single custom dictionary
type CustomDict struct {
	Keys       []CustomDictKey       `validate:"required,dive"`
	Attributes []CustomDictAttribute `validate:"required,dive"`
	Source     string                `validate:"required"`
	Layout     string                `validate:"required,oneof=hashed iptrie complex_key_hashed"`
	Dimensions []string              `validate:"required"`
	SourceType CustomDictSourceType
	S3Config   string
}

// CustomDictKey represents a single key (matching) column of a custom dictionary
type CustomDictKey struct {
	Name                 string `validate:"required,alphanum"`
	Type                 string `validate:"required,oneof=String UInt8 UInt16 UInt32 UInt64 IPv6"`
	MatchDimension       string `validate:"omitempty,alphanum"`
	MatchDimensionSuffix string `validate:"omitempty,alphanum"`
}

// CustomDictAttribute represents a single value column of a custom dictionary
type CustomDictAttribute struct {
	Name    string `validate:"required,alphanum"`
	Type    string `validate:"required,oneof=String UInt8 UInt16 UInt32 UInt64 IPv6"`
	Label   string `validate:"omitempty,alphanum"` // empty label is acceptable, in this case fallback to name
	Default string `validate:"omitempty,alphanum"`
}

type (
	// CustomDictSourceType decides how to fetch a custom dictionary
	CustomDictSourceType int
)

const (
	// SourceFile means the custom dict is served from a local file
	SourceFile CustomDictSourceType = iota
	// SourceHTTP means the custom dict is served from an HTTP endpoint
	SourceHTTP
	// SourceS3 means the custom dict is served from an S3 bucket
	SourceS3
)

var customDictSourceTypeMap = bimap.New(map[CustomDictSourceType]string{
	SourceFile: "file",
	SourceHTTP: "http",
	SourceS3:   "s3",
})

// MarshalText turns an CustomDictSourceType to Text
func (st CustomDictSourceType) MarshalText() ([]byte, error) {
	got, ok := customDictSourceTypeMap.LoadValue(st)
	if ok {
		return []byte(got), nil
	}
	return nil, errors.New("unknown field")
}

// String turns an CustomDictSourceType to string.
func (st CustomDictSourceType) String() string {
	got, _ := customDictSourceTypeMap.LoadValue(st)
	return got
}

// UnmarshalText provides an custom dict source from a string.
func (st *CustomDictSourceType) UnmarshalText(input []byte) error {
	got, ok := customDictSourceTypeMap.LoadKey(string(input))
	if ok {
		*st = got
		return nil
	}
	return errors.New("unknown provider")
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

// GetCustomDictConfig returns the custom dicts encoded in this schema
func (c *Component) GetCustomDictConfig() map[string]CustomDict {
	return c.c.CustomDictionaries
}

// DefaultCustomDictConfiguration is the default config for a CustomDict
func DefaultCustomDictConfiguration() CustomDict {
	return CustomDict{
		Layout:     "hashed",
		SourceType: SourceFile,
	}
}

// DefaultCustomDictKeyConfiguration is the default config for a CustomDictKey
func DefaultCustomDictKeyConfiguration() CustomDictKey {
	return CustomDictKey{
		Type: "String",
	}
}

// DefaultCustomDictAttributeConfiguration is the default config for a CustomDictAttribute
func DefaultCustomDictAttributeConfiguration() CustomDictAttribute {
	return CustomDictAttribute{
		Type: "String",
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.DefaultValuesUnmarshallerHook(DefaultCustomDictConfiguration()))
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.DefaultValuesUnmarshallerHook(DefaultCustomDictKeyConfiguration()))
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.DefaultValuesUnmarshallerHook(DefaultCustomDictAttributeConfiguration()))
}
