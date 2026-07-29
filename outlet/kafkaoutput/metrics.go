// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafkaoutput

import (
	"akvorado/common/reporter"
)

type metrics struct {
	messagesSent reporter.Counter
	bytesSent    reporter.Counter
	dropped      reporter.Counter
	errors       *reporter.CounterVec
}

func (c *Component) initMetrics() {
	c.metrics.messagesSent = c.r.Counter(
		reporter.CounterOpts{
			Name: "sent_messages_total",
			Help: "Number of enriched flow messages sent to Kafka.",
		},
	)
	c.metrics.bytesSent = c.r.Counter(
		reporter.CounterOpts{
			Name: "sent_bytes_total",
			Help: "Number of bytes sent to Kafka.",
		},
	)
	c.metrics.dropped = c.r.Counter(
		reporter.CounterOpts{
			Name: "dropped_messages_total",
			Help: "Number of enriched flow messages dropped because the producer buffer was full.",
		},
	)
	c.metrics.errors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Number of errors when sending to Kafka.",
		},
		[]string{"error"},
	)
}
