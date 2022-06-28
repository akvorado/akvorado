// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"sync/atomic"

	"akvorado/common/reporter"
)

type metrics struct {
	flowsReceived    *reporter.CounterVec
	flowsForwarded   *reporter.CounterVec
	flowsErrors      *reporter.CounterVec
	flowsHTTPClients reporter.GaugeFunc

	classifierCacheHits   reporter.CounterFunc
	classifierCacheMisses reporter.CounterFunc
	classifierErrors      *reporter.CounterVec
}

func (c *Component) initMetrics() {
	c.metrics.flowsReceived = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "flows_received",
			Help: "Number of incoming flows.",
		},
		[]string{"exporter"},
	)
	c.metrics.flowsForwarded = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "flows_forwarded",
			Help: "Number of flows forwarded to Kafka.",
		},
		[]string{"exporter"},
	)
	c.metrics.flowsErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "flows_errors",
			Help: "Number of flows with errors.",
		},
		[]string{"exporter", "error"},
	)
	c.metrics.flowsHTTPClients = c.r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "flows_http_clients",
			Help: "Number of HTTP clients requesting flows.",
		},
		func() float64 {
			return float64(atomic.LoadUint32(&c.httpFlowClients))
		},
	)

	c.metrics.classifierCacheHits = c.r.CounterFunc(
		reporter.CounterOpts{
			Name: "classifier_cache_hits",
			Help: "Number of hits in the classifier cache",
		},
		func() float64 {
			return float64(c.classifierCache.Metrics.Hits())
		},
	)
	c.metrics.classifierCacheMisses = c.r.CounterFunc(
		reporter.CounterOpts{
			Name: "classifier_cache_misses",
			Help: "Number of misses in the classifier cache",
		},
		func() float64 {
			return float64(c.classifierCache.Metrics.Misses())
		},
	)
	c.metrics.classifierErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "classifier_errors",
			Help: "Number of errors when evaluating a classifer",
		},
		[]string{"type", "index"})
}
