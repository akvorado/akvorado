package routing

import "akvorado/common/reporter"

type metrics struct {
	routingLookups       *reporter.CounterVec
	routingLookupsFailed *reporter.CounterVec
}

// initMetrics initialize the metrics for the BMP component.
func (c *Component) initMetrics() {
	c.metrics.routingLookups = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "routing_lookups_total",
			Help: "Number of routing lookups",
		},
		[]string{},
	)
	c.metrics.routingLookupsFailed = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "routing_failed_lookups_total",
			Help: "Number of failed routing lookups",
		},
		[]string{},
	)
}
