// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package gnmi use gNMI to get interface names and descriptions.
package gnmi

import (
	"context"
	"net/netip"
	"sync"

	"akvorado/common/reporter"
	"akvorado/outlet/metadata/provider"
)

// Provider represents the gNMI provider.
type Provider struct {
	r       *reporter.Reporter
	config  *Configuration
	metrics metrics

	state     map[netip.Addr]*exporterState
	stateLock sync.Mutex
	refresh   chan bool
}

var (
	_ provider.Provider      = &Provider{}
	_ provider.Configuration = Configuration{}
)

// New creates a new gNMI provider from configuration
func (configuration Configuration) New(r *reporter.Reporter) (provider.Provider, error) {
	p := Provider{
		r:       r,
		config:  &configuration,
		state:   map[netip.Addr]*exporterState{},
		refresh: make(chan bool),
	}
	p.initMetrics()
	return &p, nil
}

// Query queries exporter to get information through gNMI.
func (p *Provider) Query(ctx context.Context, q provider.Query) (provider.Answer, error) {
	p.stateLock.Lock()
	state, ok := p.state[q.ExporterIP]
	if !ok {
		state = &exporterState{
			Ready: make(chan bool),
		}
		p.state[q.ExporterIP] = state
		p.metrics.collectorCount.Inc()
		go p.startCollector(ctx, q.ExporterIP, state)
	}

	// Trigger a refresh
	select {
	case p.refresh <- true:
	default:
	}

	// Wait for the collector to be ready.
	select {
	case <-state.Ready:
		// Most common case, keep the lock
	default:
		// Not ready, release the lock until ready
		p.stateLock.Unlock()
		select {
		case <-state.Ready:
			p.stateLock.Lock()
		case <-ctx.Done():
			p.metrics.errors.WithLabelValues(q.ExporterIP.Unmap().String(), "not ready").Inc()
			return provider.Answer{}, ctx.Err()
		}
	}
	defer p.stateLock.Unlock()

	// Return the result from the state
	iface, ok := state.Interfaces[q.IfIndex]
	if !ok {
		return provider.Answer{}, nil
	}
	return provider.Answer{
		Found: true,
		Exporter: provider.Exporter{
			Name: state.Name,
		},
		Interface: iface,
	}, nil
}
