// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package routing

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
	"akvorado/outlet/routing/provider"
	"akvorado/outlet/routing/provider/bmp"
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

type mockProvider struct {
	lookup func(context.Context, netip.Addr, netip.Addr, netip.Addr) (provider.LookupResult, error)
}

func (m mockProvider) Lookup(ctx context.Context, ip, nh, agent netip.Addr) (provider.LookupResult, error) {
	return m.lookup(ctx, ip, nh, agent)
}

// NewCustomMock creates a new routing component with a custom lookup function.
func NewCustomMock(t *testing.T, r *reporter.Reporter, lookup func(context.Context, netip.Addr, netip.Addr, netip.Addr) (provider.LookupResult, error)) *Component {
	t.Helper()
	c := &Component{
		r:         r,
		provider:  mockProvider{lookup: lookup},
		errLogger: r.Sample(reporter.BurstSampler(time.Minute, 3)),
		config:    DefaultConfiguration(),
	}
	c.initMetrics()
	return c
}

// PopulateRIB adds some entries to the BMP provider.
func (c *Component) PopulateRIB(t *testing.T) {
	c.provider.(*bmp.Provider).PopulateRIB(t)
}
