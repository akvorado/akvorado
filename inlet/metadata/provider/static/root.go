// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package static is a metadata provider using static configuration to answer to
// requests.
package static

import (
	"akvorado/common/helpers"
	"akvorado/common/remotedatasourcefetcher"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/metadata/provider"

	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// Provider represents the static provider.
type Provider struct {
	r                      *reporter.Reporter
	exporterSourcesFetcher *remotedatasourcefetcher.Component[exporterInfo]
	exportersMap           map[string][]exporterInfo
	exporters              atomic.Pointer[helpers.SubnetMap[ExporterConfiguration]]
	exportersLock          sync.Mutex
	put                    func(provider.Update)
}

// New creates a new static provider from configuration
func (configuration Configuration) New(r *reporter.Reporter, put func(provider.Update)) (provider.Provider, error) {
	p := &Provider{
		r:            r,
		exportersMap: map[string][]exporterInfo{},
		put:          put,
	}
	p.exporters.Store(configuration.Exporters)
	p.initStaticExporters()
	var err error
	p.exporterSourcesFetcher, err = remotedatasourcefetcher.New[exporterInfo](r, p.UpdateRemoteDataSource, "metadata", configuration.ExporterSources)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize remote data source fetcher component: %w", err)
	}
	if err := p.exporterSourcesFetcher.Start(); err != nil {
		return nil, fmt.Errorf("unable to start network sources fetcher component: %w", err)
	}
	return p, nil
}

func (p *Provider) convertBoundary(boundary string) schema.InterfaceBoundary {
	switch boundary {
	case "external":
		return schema.InterfaceBoundaryExternal
	case "internal":
		return schema.InterfaceBoundaryInternal
	}
	return schema.InterfaceBoundaryUndefined
}

// Query queries static configuration.
func (p *Provider) Query(_ context.Context, query provider.BatchQuery) error {
	exporter, ok := p.exporters.Load().Lookup(query.ExporterIP)
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
				ExporterName:          exporter.Name,
				ExporterRegion:        exporter.Region,
				ExporterRole:          exporter.Role,
				ExporterTenant:        exporter.Tenant,
				ExporterSite:          exporter.Site,
				ExporterGroup:         exporter.Group,
				InterfaceName:         iface.Name,
				InterfaceDescription:  iface.Description,
				InterfaceSpeed:        iface.Speed,
				InterfaceProvider:     iface.Provider,
				InterfaceConnectivity: iface.Connectivity,
				InterfaceBoundary:     p.convertBoundary(iface.Boundary),
			},
		})
	}
	return nil
}
