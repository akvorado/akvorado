// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"sync/atomic"

	"akvorado/common/reporter"
)

type metrics struct {
	rawFlowsReceived reporter.Counter
	rawFlowsErrors   *reporter.CounterVec
	flowsReceived    *reporter.CounterVec
	flowsForwarded   *reporter.CounterVec
	flowsErrors      *reporter.CounterVec
	flowsRateLimited *reporter.CounterVec
	flowsHTTPClients reporter.GaugeFunc

	classifierExporterCacheSize  reporter.CounterFunc
	classifierInterfaceCacheSize reporter.CounterFunc
	classifierErrors             *reporter.CounterVec
}

func (c *Component) initMetrics() {
	c.metrics.rawFlowsReceived = c.r.Counter(
		reporter.CounterOpts{
			Name: "received_raw_flows_total",
			Help: "Number of incoming raw flows (proto).",
		},
	)
	c.metrics.rawFlowsErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "raw_flows_errors_total",
			Help: "Number of raw flows with errors.",
		},
		[]string{"error"},
	)
	c.metrics.flowsReceived = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "received_flows_total",
			Help: "Number of incoming flows.",
		},
		[]string{"exporter"},
	)
	c.metrics.flowsForwarded = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "forwarded_flows_total",
			Help: "Number of flows forwarded to Kafka.",
		},
		[]string{"exporter"},
	)
	c.metrics.flowsErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "flows_errors_total",
			Help: "Number of flows with errors.",
		},
		[]string{"exporter", "error"},
	)
	c.metrics.flowsRateLimited = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "flows_rate_limited_total",
			Help: "Number of flows dropped by rate limiter.",
		},
		[]string{"exporter"},
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

	c.metrics.classifierExporterCacheSize = c.r.CounterFunc(
		reporter.CounterOpts{
			Name: "classifier_exporter_cache_items_total",
			Help: "Number of items in the exporter classifier cache.",
		},
		func() float64 {
			return float64(c.classifierExporterCache.Size())
		},
	)
	c.metrics.classifierInterfaceCacheSize = c.r.CounterFunc(
		reporter.CounterOpts{
			Name: "classifier_interface_cache_items_total",
			Help: "Number of items in the interface classifier cache.",
		},
		func() float64 {
			return float64(c.classifierInterfaceCache.Size())
		},
	)
	c.metrics.classifierErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "classifier_errors_total",
			Help: "Number of errors when evaluating a classifer.",
		},
		[]string{"type", "index"})
}
