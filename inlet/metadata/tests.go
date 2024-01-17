// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package metadata

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/metadata/provider"
)

// mockProvider represents a mock provider.
type mockProvider struct {
	put func(provider.Update)
}

// Query query the mock provider for a value.
func (mp mockProvider) Query(_ context.Context, query provider.BatchQuery) error {
	for _, ifIndex := range query.IfIndexes {
		answer := provider.Answer{
			ExporterName: strings.ReplaceAll(query.ExporterIP.Unmap().String(), ".", "_"),
		}
		if ifIndex != 999 {
			answer.InterfaceName = fmt.Sprintf("Gi0/0/%d", ifIndex)
			answer.InterfaceDescription = fmt.Sprintf("Interface %d", ifIndex)
			answer.InterfaceSpeed = 1000
		}
		// in iface with  metadata (overriden by out iface)
		if ifIndex == 1010 {
			answer.ExporterGroup = "metadata group"
			answer.ExporterRegion = "metadata region"
			answer.ExporterRole = "metadata role"
			answer.ExporterSite = "metadata site"
			answer.ExporterTenant = "metadata tenant"
		}

		// out iface with metadata
		if ifIndex == 2010 {
			answer.InterfaceBoundary = schema.InterfaceBoundaryExternal
			answer.InterfaceConnectivity = "metadata connectivity"
			answer.InterfaceProvider = "metadata provider"
			answer.ExporterGroup = "metadata group"
			answer.ExporterRegion = "metadata region"
			answer.ExporterRole = "metadata role"
			answer.ExporterSite = "metadata site"
			answer.ExporterTenant = "metadata tenant"
		}
		mp.put(provider.Update{Query: provider.Query{ExporterIP: query.ExporterIP, IfIndex: ifIndex}, Answer: answer})
	}
	return nil
}

// mockProviderConfiguration is the configuration for the mock provider.
type mockProviderConfiguration struct{}

// New returns a new mock provider.
func (mpc mockProviderConfiguration) New(_ *reporter.Reporter, put func(provider.Update)) (provider.Provider, error) {
	return mockProvider{put: put}, nil
}

// NewMock creates a new metadata component building synthetic values. It is already started.
func NewMock(t *testing.T, reporter *reporter.Reporter, configuration Configuration, dependencies Dependencies) *Component {
	t.Helper()
	if configuration.Provider.Config == nil {
		configuration.Provider.Config = mockProviderConfiguration{}
	}
	c, err := New(reporter, configuration, dependencies)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)
	return c
}
