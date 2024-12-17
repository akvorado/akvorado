// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import "akvorado/common/reporter"

type metrics struct {
	batches    reporter.Counter
	flows      reporter.Counter
	waitTime   reporter.Histogram
	insertTime reporter.Histogram
	errors     *reporter.CounterVec
}

func (c *realComponent) initMetrics() {
	c.metrics.batches = c.r.Counter(
		reporter.CounterOpts{
			Name: "batches_total",
			Help: "Number of batches of flows sent to ClickHouse",
		},
	)
	c.metrics.flows = c.r.Counter(
		reporter.CounterOpts{
			Name: "flows_total",
			Help: "Number of flows sent to ClickHouse",
		},
	)
	c.metrics.waitTime = c.r.Histogram(
		reporter.HistogramOpts{
			Name: "wait_time_seconds",
			Help: "Time spent waiting before sending a batch to ClickHouse",
		},
	)
	c.metrics.insertTime = c.r.Histogram(
		reporter.HistogramOpts{
			Name: "insert_time_seconds",
			Help: "Time spent inserting data to ClickHouse",
		},
	)
	c.metrics.errors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Errors while inserting into ClickHouse",
		},
		[]string{"error"},
	)
}
