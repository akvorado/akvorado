// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package snmp

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// mockPoller will use static data.
type mockPoller struct {
	config Configuration
	put    func(string, string, uint, Interface)
}

// newMockPoller creates a fake SNMP poller.
func newMockPoller(configuration Configuration, put func(string, string, uint, Interface)) *mockPoller {
	return &mockPoller{
		config: configuration,
		put:    put,
	}
}

// Poll just builds synthetic data.
func (p *mockPoller) Poll(ctx context.Context, exporter string, port uint16, ifIndexes []uint) error {
	for _, ifIndex := range ifIndexes {
		if p.config.Communities.LookupOrDefault(net.ParseIP(exporter), "public") == "public" {
			p.put(exporter, strings.ReplaceAll(exporter, ".", "_"), ifIndex, Interface{
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
	c.poller = newMockPoller(configuration, c.sc.Put)
	helpers.StartStop(t, c)
	return c
}
