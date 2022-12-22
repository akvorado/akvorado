// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package http

import "akvorado/common/reporter"

type metrics struct {
	inflights reporter.Gauge
	requests  *reporter.CounterVec
	durations *reporter.HistogramVec
	sizes     *reporter.HistogramVec
	cacheHit  *reporter.CounterVec
	cacheMiss *reporter.CounterVec
}

func (c *Component) initMetrics() {
	c.metrics.inflights = c.r.Gauge(
		reporter.GaugeOpts{
			Name: "inflight_requests",
			Help: "Number of requests currently being served by the HTTP server.",
		},
	)
	c.metrics.requests = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "requests_total",
			Help: "Number of requests handled by an handler.",
		}, []string{"handler", "code", "method"},
	)
	c.metrics.durations = c.r.HistogramVec(
		reporter.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "Latencies for served requests.",
			Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
		}, []string{"handler", "method"},
	)
	c.metrics.sizes = c.r.HistogramVec(
		reporter.HistogramOpts{
			Name:    "response_size_bytes",
			Help:    "Response sizes for requests.",
			Buckets: []float64{200, 500, 1000, 1500, 5000},
		}, []string{"handler", "method"},
	)
	c.metrics.cacheHit = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "cache_hit_total",
			Help: "Number of requests served from cache",
		}, []string{"path", "method"},
	)
	c.metrics.cacheMiss = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "cache_miss_total",
			Help: "Number of requests not served from cache",
		}, []string{"path", "method"},
	)
}
