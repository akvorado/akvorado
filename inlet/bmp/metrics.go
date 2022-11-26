// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import "akvorado/common/reporter"

type metrics struct {
	openedConnections    *reporter.CounterVec
	closedConnections    *reporter.CounterVec
	peers                *reporter.GaugeVec
	routes               *reporter.GaugeVec
	ignoredNlri          *reporter.CounterVec
	messages             *reporter.CounterVec
	errors               *reporter.CounterVec
	ignored              *reporter.CounterVec
	panics               *reporter.CounterVec
	locked               *reporter.SummaryVec
	peerRemovalDone      *reporter.CounterVec
	peerRemovalPartial   *reporter.CounterVec
	peerRemovalQueueFull *reporter.CounterVec
}

// initMetrics initialize the metrics for the BMP component.
func (c *Component) initMetrics() {
	c.metrics.openedConnections = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "opened_connections_total",
			Help: "Number of opened connections.",
		},
		[]string{"exporter"},
	)
	c.metrics.closedConnections = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "closed_connections_total",
			Help: "Number of closed connections.",
		},
		[]string{"exporter"},
	)
	c.metrics.peers = c.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "peers_total",
			Help: "Number of peers up.",
		},
		[]string{"exporter"},
	)
	c.metrics.routes = c.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "routes_total",
			Help: "Number of routes up.",
		},
		[]string{"exporter"},
	)
	c.metrics.ignoredNlri = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "ignored_nlri_total",
			Help: "Number ignored MP NLRI received.",
		},
		[]string{"exporter", "type"},
	)
	c.metrics.messages = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "messages_received_total",
			Help: "Number of BMP messages received.",
		},
		[]string{"exporter", "type"},
	)
	c.metrics.errors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Number of fatal errors while processing BMP messages.",
		},
		[]string{"exporter", "error"},
	)
	c.metrics.ignored = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "ignored_total",
			Help: "Number of ignored BGP updates.",
		},
		[]string{"exporter", "reason", "error"},
	)
	c.metrics.panics = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "panics_total",
			Help: "Number of fatal errors while processing BMP messages.",
		},
		[]string{"exporter"},
	)
	c.metrics.locked = c.r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "locked_duration_seconds",
			Help:       "Duration during which the RIB is locked.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"reason"},
	)
	c.metrics.peerRemovalDone = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "peer_removal_done_total",
			Help: "Number of peers removed from the RIB.",
		},
		[]string{"exporter"},
	)
	c.metrics.peerRemovalPartial = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "peer_removal_partial_total",
			Help: "Number of peers partially removed from the RIB.",
		},
		[]string{"exporter"},
	)
	c.metrics.peerRemovalQueueFull = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "peer_removal_queue_full_total",
			Help: "Number of time the removal queue was full.",
		},
		[]string{"exporter"},
	)
}
