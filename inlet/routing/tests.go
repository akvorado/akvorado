// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package routing

import (
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
	"akvorado/inlet/routing/provider/bmp"
)

// NewMock creates a new mock component using BMP as a provider (it's a real one
// listening to a random port).
func NewMock(t *testing.T, r *reporter.Reporter) *Component {
	t.Helper()
	bmpConfig := bmp.DefaultConfiguration()
	bmpConfigP := bmpConfig.(bmp.Configuration)
	bmpConfigP.Listen = "127.0.0.1:0"
	config := DefaultConfiguration()
	config.Provider.Config = bmpConfigP
	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	return c
}

// PopulateRIB adds some entries to the BMP provider.
func (c *Component) PopulateRIB(t *testing.T) {
	c.provider.(*bmp.Provider).PopulateRIB(t)
}
