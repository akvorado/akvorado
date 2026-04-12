// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"akvorado/common/helpers"
)

// SkipIndexType describes a ClickHouse data-skipping index.
// Accepted forms: "minmax", "set(N)" where N >= 0, or "bloom(P)" where 0 < P < 1.
type SkipIndexType string

// ClickHouseType returns the TYPE clause string used in ALTER TABLE ... ADD INDEX.
func (s SkipIndexType) ClickHouseType() (string, error) {
	str := string(s)
	switch {
	case str == "minmax":
		return "minmax", nil
	case strings.HasPrefix(str, "set(") && strings.HasSuffix(str, ")"):
		inner := str[4 : len(str)-1]
		n, err := strconv.Atoi(inner)
		if err != nil || n < 0 {
			return "", fmt.Errorf("invalid set index %q: argument must be a non-negative integer", s)
		}
		return str, nil
	case strings.HasPrefix(str, "bloom(") && strings.HasSuffix(str, ")"):
		inner := str[6 : len(str)-1]
		p, err := strconv.ParseFloat(inner, 64)
		if err != nil || p <= 0 || p >= 1 {
			return "", fmt.Errorf("invalid bloom index %q: FPP must be in (0, 1)", s)
		}
		return fmt.Sprintf("bloom_filter(%g)", p), nil
	default:
		return "", fmt.Errorf("unknown skip index type %q: use minmax, set(N), or bloom(P)", s)
	}
}

// UnmarshalText validates and sets a SkipIndexType from its text representation.
func (s *SkipIndexType) UnmarshalText(input []byte) error {
	v := SkipIndexType(input)
	if _, err := v.ClickHouseType(); err != nil {
		return err
	}
	*s = v
	return nil
}

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
	// Indexes maps column names to the desired ClickHouse data-skipping index.
	// Accepted values: "minmax", "set(N)" (N >= 0), or "bloom(P)" (0 < P < 1).
	// Indexes are applied only to the main flows table. Entries here are merged
	// with (and override) the defaults; use NoIndexes to remove a default.
	Indexes map[ColumnKey]SkipIndexType
	// NoIndexes lists columns whose default skip index should be removed.
	NoIndexes []ColumnKey `validate:"ninterfield=Indexes"`
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

// DefaultIndexes are the skip indexes applied when none are explicitly configured.
var DefaultIndexes = map[ColumnKey]SkipIndexType{
	ColumnSrcAddr:           "bloom(0.001)",
	ColumnDstAddr:           "bloom(0.001)",
	ColumnSrcAS:             "bloom(0.001)",
	ColumnDstAS:             "bloom(0.001)",
	ColumnSrcPort:           "bloom(0.001)",
	ColumnDstPort:           "bloom(0.001)",
	ColumnSrcCountry:        "bloom(0.001)",
	ColumnDstCountry:        "bloom(0.001)",
	ColumnExporterName:      "minmax",
	ColumnInIfProvider:      "set(0)",
	ColumnOutIfProvider:     "set(0)",
	ColumnInIfConnectivity:  "set(0)",
	ColumnOutIfConnectivity: "set(0)",
	ColumnInIfBoundary:      "set(0)",
	ColumnOutIfBoundary:     "set(0)",
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

// GetSkipIndexes returns the configured data-skipping indexes.
func (c *Component) GetSkipIndexes() map[ColumnKey]SkipIndexType {
	return c.c.Indexes
}

// DefaultCustomDictConfiguration is the default config for a CustomDict
func DefaultCustomDictConfiguration() CustomDict {
	return CustomDict{
		Layout: "hashed",
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
