// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package gnmi use gNMI to get interface names and descriptions.
package gnmi

import (
	"context"
	"net/netip"
	"sync"

	"akvorado/common/reporter"
	"akvorado/inlet/metadata/provider"
)

// Provider represents the gNMI provider.
type Provider struct {
	r       *reporter.Reporter
	config  *Configuration
	metrics metrics

	put     func(provider.Update)
	refresh chan bool

	state     map[netip.Addr]*exporterState
	stateLock sync.Mutex
}

// New creates a new gNMI provider from configuration
func (configuration Configuration) New(r *reporter.Reporter, put func(provider.Update)) (provider.Provider, error) {
	p := Provider{
		r:       r,
		config:  &configuration,
		put:     put,
		state:   map[netip.Addr]*exporterState{},
		refresh: make(chan bool),
	}
	p.initMetrics()
	return &p, nil
}

// Query queries exporter to get information through gNMI.
func (p *Provider) Query(ctx context.Context, q provider.BatchQuery) error {
	p.stateLock.Lock()
	defer p.stateLock.Unlock()
	state, ok := p.state[q.ExporterIP]
	// If we don't have a collector for the provided IP, starts one. We should
	// be sure we don't have several collectors for the same exporter, hence the
	// write lock for everything.
	if !ok {
		state := exporterState{}
		p.state[q.ExporterIP] = &state
		go p.startCollector(ctx, q.ExporterIP, &state)
		p.metrics.collectorCount.Inc()
		return nil
	}
	// If the collector exists and already provided some data, populate the
	// cache.
	if state.Ready {
		for _, ifindex := range q.IfIndexes {
			p.put(provider.Update{
				Query: provider.Query{
					ExporterIP: q.ExporterIP,
					IfIndex:    ifindex,
				},
				Answer: provider.Answer{
					Exporter: provider.Exporter{
						Name: state.Name,
					},
					Interface: state.Interfaces[ifindex],
				},
			})
		}
		// Also trigger a refresh
		select {
		case p.refresh <- true:
		default:
		}
	}
	return nil
}
