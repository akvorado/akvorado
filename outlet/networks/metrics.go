// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package networks

import "akvorado/common/reporter"

type metrics struct {
	rebuilds        reporter.Counter
	rebuildTime     reporter.Counter
	rebuildLastTime reporter.Gauge
	prefixes        reporter.Gauge
}

// initMetrics initialize the metrics for the networks component.
func (c *Component) initMetrics() {
	c.metrics.rebuilds = c.r.Counter(
		reporter.CounterOpts{
			Name: "rebuilds_total",
			Help: "Number of times the networks were rebuilt.",
		},
	)
	c.metrics.rebuildTime = c.r.Counter(
		reporter.CounterOpts{
			Name: "rebuild_time_seconds_total",
			Help: "Cumulated time spent rebuilding the networks.",
		},
	)
	c.metrics.rebuildLastTime = c.r.Gauge(
		reporter.GaugeOpts{
			Name: "rebuild_last_time_seconds",
			Help: "Time spent during the last rebuild of the networks.",
		},
	)
	c.metrics.prefixes = c.r.Gauge(
		reporter.GaugeOpts{
			Name: "prefixes",
			Help: "Number of prefixes, including the ones from the GeoIP databases.",
		},
	)
	c.r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "memory_bytes",
			Help: "Estimated memory taken by the prefixes and their attributes.",
		},
		func() float64 { return float64(measureMemory()) },
	)
}
