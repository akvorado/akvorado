// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package tcp

import "akvorado/common/reporter"

type metrics struct {
	bytes             *reporter.CounterVec
	messages          *reporter.CounterVec
	errors            *reporter.CounterVec
	openedConnections *reporter.CounterVec
	closedConnections *reporter.CounterVec
	connections       *reporter.GaugeVec
}

// initMetrics initializes the metrics for the TCP input.
func (in *Input) initMetrics() {
	in.metrics.bytes = in.r.CounterVec(
		reporter.CounterOpts{
			Name: "bytes_total",
			Help: "Bytes received by the application.",
		},
		[]string{"listener", "exporter"},
	)
	in.metrics.messages = in.r.CounterVec(
		reporter.CounterOpts{
			Name: "messages_total",
			Help: "Messages received by the application.",
		},
		[]string{"listener", "exporter"},
	)
	in.metrics.errors = in.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Errors while receiving messages.",
		},
		[]string{"listener", "exporter"},
	)
	in.metrics.openedConnections = in.r.CounterVec(
		reporter.CounterOpts{
			Name: "opened_connections_total",
			Help: "Number of opened connections.",
		},
		[]string{"listener", "exporter"},
	)
	in.metrics.closedConnections = in.r.CounterVec(
		reporter.CounterOpts{
			Name: "closed_connections_total",
			Help: "Number of closed connections.",
		},
		[]string{"listener", "exporter"},
	)

	in.metrics.connections = in.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "connections",
			Help: "Number of current connections.",
		},
		[]string{"listener", "exporter"},
	)
}
