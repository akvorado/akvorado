// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package schema is an abstraction of the data schema for flows used by
// Akvorado. It is a leaky abstraction as there are multiple parts dependant of
// the subsystem that will use it.
package schema

// Component represents the schema compomenent.
type Component struct {
	Schema
}

// New creates a new schema component.
func New() (*Component, error) {
	return &Component{
		Schema: flows(),
	}, nil
}
