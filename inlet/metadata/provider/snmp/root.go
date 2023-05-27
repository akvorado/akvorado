// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package snmp handles SNMP polling to get interface names and
// descriptions. It keeps a cache of retrieved entries and refresh
// them.
package snmp

import (
	"context"
	"net/netip"
	"sync"
	"time"

	"akvorado/common/reporter"
	"akvorado/inlet/metadata/provider"
)

// Provider represents the SNMP provider.
type Provider struct {
	r      *reporter.Reporter
	config *Configuration

	pendingRequests     map[string]struct{}
	pendingRequestsLock sync.Mutex
	errLogger           reporter.Logger

	metrics struct {
		pendingRequests reporter.GaugeFunc
		successes       *reporter.CounterVec
		errors          *reporter.CounterVec
		retries         *reporter.CounterVec
		times           *reporter.SummaryVec
	}
}

// New creates a new SNMP provider from configuration
func (configuration Configuration) New(r *reporter.Reporter) (provider.Provider, error) {
	for exporterIP, agentIP := range configuration.Agents {
		if exporterIP.Is4() || agentIP.Is4() {
			delete(configuration.Agents, exporterIP)
			exporterIP = netip.AddrFrom16(exporterIP.As16())
			agentIP = netip.AddrFrom16(agentIP.As16())
			configuration.Agents[exporterIP] = agentIP
		}
	}

	p := Provider{
		r:      r,
		config: &configuration,

		pendingRequests: make(map[string]struct{}),
		errLogger:       r.Sample(reporter.BurstSampler(10*time.Second, 3)),
	}

	p.metrics.pendingRequests = r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "poller_pending_requests",
			Help: "Number of pending requests in pollers.",
		}, func() float64 {
			p.pendingRequestsLock.Lock()
			defer p.pendingRequestsLock.Unlock()
			return float64(len(p.pendingRequests))
		})
	p.metrics.successes = r.CounterVec(
		reporter.CounterOpts{
			Name: "poller_success_requests",
			Help: "Number of successful requests.",
		}, []string{"exporter"})
	p.metrics.errors = r.CounterVec(
		reporter.CounterOpts{
			Name: "poller_error_requests",
			Help: "Number of failed requests.",
		}, []string{"exporter", "error"})
	p.metrics.retries = r.CounterVec(
		reporter.CounterOpts{
			Name: "poller_retry_requests",
			Help: "Number of retried requests.",
		}, []string{"exporter"})
	p.metrics.times = r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "poller_seconds",
			Help:       "Time to successfully poll for values.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}, []string{"exporter"})

	return &p, nil
}

// Query queries exporter to get information through SNMP.
func (p *Provider) Query(ctx context.Context, query provider.Query, put func(provider.Update)) error {
	// Avoid querying too much exporters with errors
	agentIP, ok := p.config.Agents[query.ExporterIP]
	if !ok {
		agentIP = query.ExporterIP
	}
	agentPort := p.config.Ports.LookupOrDefault(agentIP, 161)
	return p.Poll(ctx, query.ExporterIP, agentIP, agentPort, []uint{query.IfIndex}, put)
}
