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
	for _, k1 := range config.Enabled {
		for _, k2 := range config.Disabled {
			if k1 == k2 {
				return nil, fmt.Errorf("column %q contained in both EnabledColumns and DisabledColumns", k1)
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
	return &Component{
		c:      config,
		Schema: schema.finalize(),
	}, nil
}
