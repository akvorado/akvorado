// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import "akvorado/common/reporter"

type metrics struct {
	flows      reporter.Summary
	waitTime   reporter.Histogram
	insertTime reporter.Histogram
	errors     *reporter.CounterVec
}

func (c *realComponent) initMetrics() {
	c.metrics.flows = c.r.Summary(
		reporter.SummaryOpts{
			Name:       "flow_per_batch",
			Help:       "Number of flow per batch sent to ClickHouse",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
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
