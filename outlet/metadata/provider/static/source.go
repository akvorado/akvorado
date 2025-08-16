// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package static

import (
	"context"
	"errors"

	"akvorado/common/helpers"
	"akvorado/common/remotedatasource"
	"akvorado/outlet/metadata/provider"
)

type exporterInfo struct {
	provider.Exporter `mapstructure:",squash" yaml:",inline"`
	ExporterSubnet    string
	// Default is used if not empty for any unknown ifindexes
	Default provider.Interface `validate:"omitempty"`
	// IfIndexes is a map from interface indexes to interfaces
	Interfaces []exporterInterface `validate:"omitempty"`
}

type exporterInterface struct {
	IfIndex            uint
	provider.Interface `validate:"omitempty,dive" mapstructure:",squash"`
}

func (i exporterInfo) toExporterConfiguration() ExporterConfiguration {
	ifindexMap := map[uint]provider.Interface{}
	for _, iface := range i.Interfaces {
		ifindexMap[iface.IfIndex] = iface.Interface
	}

	return ExporterConfiguration{
		Exporter:  i.Exporter,
		Default:   i.Default,
		IfIndexes: ifindexMap,
	}
}

// initStaticExporters initializes the reconciliation map for exporter configurations
// with the static prioritized data from exporters' Configuration.
func (p *Provider) initStaticExporters() {
	staticExporters := make([]exporterInfo, 0)
	staticExportersMap := p.exporters.Load()
	for subnet, config := range staticExportersMap.All() {
		interfaces := make([]exporterInterface, 0, len(config.IfIndexes))
		for ifindex, iface := range config.IfIndexes {
			interfaces = append(interfaces, exporterInterface{
				IfIndex:   ifindex,
				Interface: iface,
			})
		}
		staticExporters = append(
			staticExporters,
			exporterInfo{
				Exporter: provider.Exporter{
					Name: config.Name,
				},
				ExporterSubnet: subnet.String(),
				Default:        config.Default,
				Interfaces:     interfaces,
			},
		)
	}
	p.exportersMap["static"] = staticExporters
}

// UpdateSource updates a remote metadata exporters source. It returns the
// number of exporters retrieved.
func (p *Provider) UpdateSource(ctx context.Context, name string, source remotedatasource.Source) (int, error) {
	results, err := p.exporterSourcesFetcher.Fetch(ctx, name, source)
	if err != nil {
		return 0, err
	}
	finalMap := map[string]ExporterConfiguration{}
	p.exportersLock.Lock()
	p.exportersMap[name] = results
	for id, results := range p.exportersMap {
		if id == "static" {
			continue
		}
		for _, exporterData := range results {
			exporterSubnet, err := helpers.SubnetMapParseKey(exporterData.ExporterSubnet)
			if err != nil {
				p.r.Err(err).Msg("failed to decode subnet")
				continue
			}
			// Concurrency for same Exporter config across multiple remote data sources is not handled
			finalMap[exporterSubnet.String()] = exporterData.toExporterConfiguration()
		}
	}
	for _, exporterData := range p.exportersMap["static"] {
		exporterSubnet, err := helpers.SubnetMapParseKey(exporterData.ExporterSubnet)
		if err != nil {
			p.r.Err(err).Msg("failed to decode subnet")
			continue
		}
		// This overrides duplicates config for an Exporter if it's also defined as static
		finalMap[exporterSubnet.String()] = exporterData.toExporterConfiguration()
	}
	p.exportersLock.Unlock()
	exporters, err := helpers.NewSubnetMap(finalMap)
	if err != nil {
		return 0, errors.New("cannot create subnetmap")
	}
	p.exporters.Swap(exporters)
	return len(results), nil
}
