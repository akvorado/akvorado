// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package httpserver

import (
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// NewMock create a new HTTP component listening on a random free port.
func NewMock(t *testing.T, r *reporter.Reporter) *Component {
	t.Helper()
	config := DefaultConfiguration()
	config.Listen = "0.0.0.0:0"
	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)
	return c
}
