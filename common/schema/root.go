// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package schema is an abstraction of the data schema for flows used by
// Akvorado. It is a leaky abstraction as there are multiple parts dependant of
// the subsystem that will use it.
package schema

import (
	"fmt"

	"golang.org/x/exp/slices"
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
			} else {
				return nil, fmt.Errorf("no alias configured for %s that can be converted to generate", k)
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
			if slices.Contains(schema.clickHousePrimaryKeys, k) {
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
			if slices.Contains(schema.clickHousePrimaryKeys, k) {
				// Primary keys are part of the sorting key.
				return nil, fmt.Errorf("column %q cannot be present on main table only (primary key)", k)
			}
			column.ClickHouseMainOnly = true
		}
	}
	return &Component{
		c:      config,
		Schema: schema.finalize(),
	}, nil
}
