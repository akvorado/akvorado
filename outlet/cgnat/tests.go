// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package cgnat

import (
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
)

// NewMock creates a mock CGNAT component with defaults.
func NewMock(t *testing.T, r *reporter.Reporter) *Component {
	t.Helper()
	c, err := New(r, DefaultConfiguration(), Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	return c
}
