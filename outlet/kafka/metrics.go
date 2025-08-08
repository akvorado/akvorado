// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"akvorado/common/reporter"
)

type metrics struct {
	messagesReceived *reporter.CounterVec
	fetchesReceived  *reporter.CounterVec
	bytesReceived    *reporter.CounterVec
	errorsReceived   *reporter.CounterVec
	workers          reporter.GaugeFunc
	workerIncrease   reporter.Counter
	workerDecrease   reporter.Counter
}

func (c *realComponent) initMetrics() {
	c.metrics.messagesReceived = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "received_messages_total",
			Help: "Number of messages received for a given worker.",
		},
		[]string{"worker"},
	)
	c.metrics.fetchesReceived = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "received_fetches_total",
			Help: "Number of fetches received for a given worker.",
		},
		[]string{"worker"},
	)
	c.metrics.bytesReceived = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "received_bytes_total",
			Help: "Number of bytes received for a given worker.",
		},
		[]string{"worker"},
	)
	c.metrics.errorsReceived = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "received_errors_total",
			Help: "Number of errors while handling received messages for a given worker.",
		},
		[]string{"worker"},
	)
	c.metrics.workers = c.r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "workers",
			Help: "Number of running workers",
		},
		func() float64 {
			c.workerMu.Lock()
			defer c.workerMu.Unlock()
			return float64(len(c.workers))
		},
	)
	c.metrics.workerIncrease = c.r.Counter(
		reporter.CounterOpts{
			Name: "worker_increase_total",
			Help: "Number of times a new worker was spawned.",
		},
	)
	c.metrics.workerDecrease = c.r.Counter(
		reporter.CounterOpts{
			Name: "worker_decrease_total",
			Help: "Number of times a new worker was stopped.",
		},
	)
}
