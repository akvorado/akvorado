// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package static is a metadata provider using static configuration to answer to
// requests.
package static

import (
	"context"

	"akvorado/common/reporter"
	"akvorado/inlet/metadata/provider"
)

// Provider represents the static provider.
type Provider struct {
	r      *reporter.Reporter
	config *Configuration
	put    func(provider.Update)
}

// New creates a new static provider from configuration
func (configuration Configuration) New(r *reporter.Reporter, put func(provider.Update)) (provider.Provider, error) {
	return &Provider{
		r:      r,
		config: &configuration,
		put:    put,
	}, nil
}

// Query queries static configuration.
func (p *Provider) Query(_ context.Context, query provider.BatchQuery) error {
	exporter, ok := p.config.Exporters.Lookup(query.ExporterIP)
	if !ok {
		return nil
	}
	for _, ifIndex := range query.IfIndexes {
		iface, ok := exporter.IfIndexes[ifIndex]
		if !ok {
			iface = exporter.Default
		}
		p.put(provider.Update{
			Query: provider.Query{
				ExporterIP: query.ExporterIP,
				IfIndex:    ifIndex,
			},
			Answer: provider.Answer{
				ExporterName: exporter.Name,
				Interface:    iface,
			},
		})
	}
	return nil
}
