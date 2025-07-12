// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"akvorado/common/kafka"
	"akvorado/common/reporter"
)

type metrics struct {
	messagesSent *reporter.CounterVec
	bytesSent    *reporter.CounterVec
	errors       *reporter.CounterVec

	kafkaMetrics kafka.Metrics
}

func (c *Component) initMetrics() {
	c.metrics.messagesSent = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "sent_messages_total",
			Help: "Number of messages sent from a given exporter.",
		},
		[]string{"exporter"},
	)
	c.metrics.bytesSent = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "sent_bytes_total",
			Help: "Number of bytes sent from a given exporter.",
		},
		[]string{"exporter"},
	)
	c.metrics.errors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Number of errors when sending.",
		},
		[]string{"error"},
	)

	c.metrics.kafkaMetrics.Init(c.r, c.kafkaConfig.MetricRegistry)
}
