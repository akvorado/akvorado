// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package dns

import "akvorado/common/reporter"

type metrics struct {
	cacheHits     reporter.Counter
	cacheMisses   reporter.Counter
	queries       reporter.Counter
	errors        *reporter.CounterVec
	queryDuration reporter.Summary
	pending       reporter.GaugeFunc
	cacheEntries  reporter.GaugeFunc
}

func (c *Component) initMetrics() {
	c.metrics.cacheHits = c.r.Counter(
		reporter.CounterOpts{
			Name: "cache_hits_total",
			Help: "Number of reverse DNS cache hits.",
		},
	)
	c.metrics.cacheMisses = c.r.Counter(
		reporter.CounterOpts{
			Name: "cache_misses_total",
			Help: "Number of reverse DNS cache misses.",
		},
	)
	c.metrics.queries = c.r.Counter(
		reporter.CounterOpts{
			Name: "queries_total",
			Help: "Number of reverse DNS queries sent.",
		},
	)
	c.metrics.errors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Number of reverse DNS resolver errors.",
		},
		[]string{"error"},
	)
	c.metrics.queryDuration = c.r.Summary(
		reporter.SummaryOpts{
			Name:       "query_duration_seconds",
			Help:       "Duration of reverse DNS queries.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
	)
	c.metrics.pending = c.r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "pending_queries",
			Help: "Number of queued or in-flight reverse DNS queries.",
		},
		func() float64 {
			c.pendingMu.Lock()
			defer c.pendingMu.Unlock()
			return float64(len(c.pending))
		},
	)
	c.metrics.cacheEntries = c.r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "cache_entries",
			Help: "Number of reverse DNS cache entries.",
		},
		func() float64 {
			c.cacheMu.Lock()
			defer c.cacheMu.Unlock()
			return float64(len(c.cache))
		},
	)
}
