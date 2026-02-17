// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import "akvorado/common/reporter"

type metrics struct {
	openedConnections  *reporter.CounterVec
	closedConnections  *reporter.CounterVec
	peers              *reporter.GaugeVec
	routes             *reporter.GaugeVec
	bufferSize         *reporter.GaugeVec
	ignoredNlri        *reporter.CounterVec
	messages           *reporter.CounterVec
	errors             *reporter.CounterVec
	ignored            *reporter.CounterVec
	panics             *reporter.CounterVec
	locked             *reporter.SummaryVec
	peerRemovalDone    *reporter.CounterVec
	messageQueueLength *reporter.GaugeVec
}

// initMetrics initialize the metrics for the BMP component.
func (p *Provider) initMetrics() {
	p.metrics.openedConnections = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "opened_connections_total",
			Help: "Number of opened connections.",
		},
		[]string{"exporter"},
	)
	p.metrics.closedConnections = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "closed_connections_total",
			Help: "Number of closed connections.",
		},
		[]string{"exporter"},
	)
	p.metrics.peers = p.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "peers",
			Help: "Number of peers up.",
		},
		[]string{"exporter"},
	)
	p.metrics.routes = p.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "routes",
			Help: "Number of routes up.",
		},
		[]string{"exporter"},
	)
	p.metrics.bufferSize = p.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "buffer_size_bytes",
			Help: "Size of the in-kernel buffer for this connection.",
		},
		[]string{"exporter"},
	)
	p.metrics.ignoredNlri = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "ignored_nlri_total",
			Help: "Number ignored MP NLRI received.",
		},
		[]string{"exporter", "type"},
	)
	p.metrics.messages = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "received_messages_total",
			Help: "Number of BMP messages received.",
		},
		[]string{"exporter", "type"},
	)
	p.metrics.errors = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Number of fatal errors while processing BMP messages.",
		},
		[]string{"exporter", "error"},
	)
	p.metrics.ignored = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "ignored_updates_total",
			Help: "Number of ignored BGP updates.",
		},
		[]string{"exporter", "error"},
	)
	p.metrics.panics = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "panics_total",
			Help: "Number of fatal errors while processing BMP messages.",
		},
		[]string{"exporter"},
	)
	p.metrics.locked = p.r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "locked_duration_seconds",
			Help:       "Duration during which the RIB is locked.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"reason"},
	)
	p.metrics.peerRemovalDone = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "removed_peers_total",
			Help: "Number of peers removed from the RIB.",
		},
		[]string{"exporter"},
	)
	p.metrics.messageQueueLength = p.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "message_queue_length",
			Help: "Number of BMP messages waiting in the processing queue.",
		},
		[]string{"exporter"},
	)
}
