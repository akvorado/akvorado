// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package static is a metadata provider using static configuration to answer to
// requests.
package static

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/remotedatasource"
	"akvorado/common/reporter"
	"akvorado/outlet/metadata/provider"
)

// Provider represents the static provider.
type Provider struct {
	r *reporter.Reporter

	exporterSourcesFetcher *remotedatasource.Component[exporterInfo]
	exportersMap           map[string][]exporterInfo
	exporters              atomic.Pointer[helpers.SubnetMap[ExporterConfiguration]]
	exportersLock          sync.Mutex

	startOnce sync.Once
	errLogger reporter.Logger

	metrics struct {
		notReady reporter.Counter
	}
}

var (
	_ provider.Provider      = &Provider{}
	_ provider.Configuration = Configuration{}
)

// New creates a new static provider from configuration
func (configuration Configuration) New(r *reporter.Reporter) (provider.Provider, error) {
	p := &Provider{
		r:            r,
		exportersMap: map[string][]exporterInfo{},
		errLogger:    r.Sample(reporter.BurstSampler(time.Minute, 3)),
	}
	p.exporters.Store(configuration.Exporters)
	p.initStaticExporters()

	var err error
	p.exporterSourcesFetcher, err = remotedatasource.New[exporterInfo](r,
		p.UpdateSource, "metadata", configuration.ExporterSources)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize remote data source fetcher component: %w", err)
	}

	p.metrics.notReady = r.Counter(
		reporter.CounterOpts{
			Name: "not_ready_total",
			Help: "Number of queries failing because the remote data sources are not ready",
		})

	return p, nil
}

// Query queries static configuration.
func (p *Provider) Query(ctx context.Context, query provider.Query) (provider.Answer, error) {
	// Start datasource if not started. It cannot really return an error.
	p.startOnce.Do(func() {
		p.exporterSourcesFetcher.Start()
	})

	// We wait for all data sources to be ready
	select {
	case <-ctx.Done():
		p.metrics.notReady.Inc()
		p.errLogger.Warn().Msg("remote datasources are not ready")
		return provider.Answer{}, ctx.Err()
	case <-p.exporterSourcesFetcher.DataSourcesReady:
	}
	exporter, ok := p.exporters.Load().Lookup(query.ExporterIP)
	if !ok {
		return provider.Answer{}, provider.ErrSkipProvider
	}

	iface, ok := exporter.IfIndexes[query.IfIndex]
	if !ok {
		if exporter.SkipMissingInterfaces {
			return provider.Answer{}, provider.ErrSkipProvider
		}
		iface = exporter.Default
	}
	return provider.Answer{
		Found:     true,
		Exporter:  exporter.Exporter,
		Interface: iface,
	}, nil
}
