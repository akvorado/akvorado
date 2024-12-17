// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package routing

import "akvorado/common/reporter"

type metrics struct {
	routingLookups       reporter.Counter
	routingLookupsFailed reporter.Counter
}

// initMetrics initialize the metrics for the BMP component.
func (c *Component) initMetrics() {
	c.metrics.routingLookups = c.r.Counter(
		reporter.CounterOpts{
			Name: "routing_lookups_total",
			Help: "Number of routing lookups",
		},
	)
	c.metrics.routingLookupsFailed = c.r.Counter(
		reporter.CounterOpts{
			Name: "routing_failed_lookups_total",
			Help: "Number of failed routing lookups",
		},
	)
}
