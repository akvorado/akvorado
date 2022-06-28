// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package authentication

import (
	"testing"

	"akvorado/common/reporter"
)

// NewMock instantiantes a new authentication component
func NewMock(t *testing.T, r *reporter.Reporter) *Component {
	t.Helper()
	c, err := New(r, DefaultConfiguration())
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	return c
}
