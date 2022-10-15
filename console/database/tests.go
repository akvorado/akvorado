// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package database

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// NewMock instantiantes a new authentication component
func NewMock(t *testing.T, r *reporter.Reporter, config Configuration) *Component {
	t.Helper()
	c, err := New(r, config)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)
	return c
}
