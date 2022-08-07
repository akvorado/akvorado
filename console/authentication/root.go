// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package authentication handles user authentication for the console.
package authentication

import "akvorado/common/reporter"

// Component represents the authentication compomenent.
type Component struct {
	r      *reporter.Reporter
	config Configuration
}

// New creates a new authentication component.
func New(r *reporter.Reporter, configuration Configuration) (*Component, error) {
	c := Component{
		r:      r,
		config: configuration,
	}

	return &c, nil
}
