// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package schema is an abstraction of the data schema for flows used by
// Akvorado. It is a leaky abstraction as there are multiple parts dependant of
// the subsystem that will use it.
package schema

import (
	"fmt"
	"slices"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Component represents the schema compomenent.
type Component struct {
	c Configuration

	Schema
}

// New creates a new schema component.
func New(config Configuration) (*Component, error) {
	schema := flows()
	for _, k := range config.Materialize {
		if column, ok := schema.LookupColumnByKey(k); ok {
			if column.ClickHouseAlias != "" {
				column.ClickHouseGenerateFrom = column.ClickHouseAlias
				column.ClickHouseAlias = ""
				column.ClickHouseMaterialized = true
			} else {
				return nil, fmt.Errorf("no alias configured for %s that can be converted to generate", k)
			}

			// In case we have another data type for materialized columns, set it
			if column.ClickHouseMaterializedType != "" {
				column.ClickHouseType = column.ClickHouseMaterializedType
			}
		}
	}
	for _, k := range config.Enabled {
		if column, ok := schema.LookupColumnByKey(k); ok {
			column.Disabled = false
		}
	}
	for _, k := range config.Disabled {
		if column, ok := schema.LookupColumnByKey(k); ok {
			if column.NoDisable {
				return nil, fmt.Errorf("column %q cannot be disabled", k)
			}
			if slices.Contains(schema.clickhousePrimaryKeys, k) {
				return nil, fmt.Errorf("column %q cannot be disabled (primary key)", k)
			}
			column.Disabled = true
		}
	}
	for _, k := range config.Disabled {
		if column, ok := schema.LookupColumnByKey(k); ok {
			for _, depend := range column.Depends {
				if ocolumn, _ := schema.LookupColumnByKey(depend); !ocolumn.Disabled {
					return nil, fmt.Errorf("column %q cannot be disabled without disabling %q", k, depend)
				}
			}
		}
	}
	for _, k := range config.NotMainTableOnly {
		if column, ok := schema.LookupColumnByKey(k); ok {
			column.ClickHouseMainOnly = false
		}
	}
	for _, k := range config.MainTableOnly {
		if column, ok := schema.LookupColumnByKey(k); ok {
			if column.NoDisable {
				return nil, fmt.Errorf("column %q cannot be present on main table only", k)
			}
			if slices.Contains(schema.clickhousePrimaryKeys, k) {
				// Primary keys are part of the sorting key.
				return nil, fmt.Errorf("column %q cannot be present on main table only (primary key)", k)
			}
			column.ClickHouseMainOnly = true
		}
	}

	// Add new columns from custom dictionaries after the static ones as we dont
	// reference the dicts in the code and they are created during runtime from
	// the config, this is enough for us.
	customDictColumns := []Column{}
	for dname, v := range config.CustomDictionaries {
		for _, d := range v.Dimensions {
			// Check if we can actually create the dictionary (we need to know what to match on)
			if len(v.Keys) == 0 {
				return nil, fmt.Errorf("custom dictionary %s has no keys, this is not supported", dname)
			}
			if len(v.Keys) > 1 {
				// If more than one key is present, every key needs either a MatchDimension or a MatchDimensionSuffix
				for _, kv := range v.Keys {
					if kv.MatchDimension == "" && kv.MatchDimensionSuffix == "" {
						return nil, fmt.Errorf("custom dictionary %s has more than one key, but key %s has neither MatchDimension nor MatchDimensionSuffix set", dname, kv.Name)
					}
				}
			}
			// First, we need to build the matching string for this
			matchingList := []string{}
			// Prefer match dimension or match dimension suffix if available
			for _, kv := range v.Keys {
				if kv.MatchDimension != "" {
					matchingList = append(matchingList, kv.MatchDimension)
					continue
				}
				// Match post is appended after the dimension name, and useful
				// if we wanna match a subkey e.g. both in Src/Dst
				if kv.MatchDimensionSuffix != "" {
					matchingList = append(matchingList, fmt.Sprintf("%s%s", d, kv.MatchDimensionSuffix))
				}
			}
			matchingString := ""
			if len(matchingList) > 0 {
				matchingString = fmt.Sprintf("(%s)", strings.Join(matchingList, ","))
			} else {
				// If match dimension and match dimension suffix are both not
				// available, we use the dimension name (e.g. SrcAddr)
				matchingString = d
			}

			for _, a := range v.Attributes {
				// Add the dimension combined with capitalizing the name of the dimension field
				l := a.Label
				if l == "" {
					l = cases.Title(language.Und).String(a.Name)
				}
				name := fmt.Sprintf("%s%s", d, l)
				key := ColumnLast + schema.dynamicColumns
				parserType := ""
				switch a.Type {
				case "IPv6":
					parserType = "ip"
				case "String":
					parserType = "string"
				case "UInt8", "UInt16", "UInt32", "UInt64":
					parserType = "uint"
				}
				customDictColumns = append(customDictColumns,
					Column{
						Key:            key,
						Name:           name,
						ParserType:     parserType,
						ClickHouseType: fmt.Sprintf("LowCardinality(%s)", a.Type),
						ClickHouseGenerateFrom: fmt.Sprintf("dictGet('custom_dict_%s', '%s', %s)", dname, a.Name,
							matchingString),
					})
				columnNameMap.Insert(key, name)
				schema.dynamicColumns++
			}
		}
	}

	schema.columns = append(schema.columns, customDictColumns...)

	return &Component{
		c:      config,
		Schema: schema.finalize(),
	}, nil
}
