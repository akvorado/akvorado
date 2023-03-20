// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package snmp

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// mockPoller will use static data.
type mockPoller struct {
	config Configuration
	put    func(netip.Addr, string, uint, Interface)
}

// newMockPoller creates a fake SNMP poller.
func newMockPoller(configuration Configuration, put func(netip.Addr, string, uint, Interface)) *mockPoller {
	return &mockPoller{
		config: configuration,
		put:    put,
	}
}

// Poll just builds synthetic data.
func (p *mockPoller) Poll(_ context.Context, exporter, _ netip.Addr, _ uint16, ifIndexes []uint) error {
	for _, ifIndex := range ifIndexes {
		if p.config.Communities.LookupOrDefault(exporter, "public") == "public" {
			p.put(exporter, strings.ReplaceAll(exporter.Unmap().String(), ".", "_"), ifIndex, Interface{
				Name:        fmt.Sprintf("Gi0/0/%d", ifIndex),
				Description: fmt.Sprintf("Interface %d", ifIndex),
				Speed:       1000,
			})
		}
	}
	return nil
}

// NewMock creates a new SNMP component building synthetic values. It is already started.
func NewMock(t *testing.T, reporter *reporter.Reporter, configuration Configuration, dependencies Dependencies) *Component {
	t.Helper()
	c, err := New(reporter, configuration, dependencies)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	// Change the poller to a fake one.
	c.poller = newMockPoller(configuration, func(ip netip.Addr, exporterName string, index uint, iface Interface) {
		if index != 999 {
			c.sc.Put(c.d.Clock.Now(), ip, exporterName, index, iface)
		} else {
			c.sc.Put(c.d.Clock.Now(), ip, exporterName, index, Interface{})
		}
	})
	helpers.StartStop(t, c)
	return c
}
