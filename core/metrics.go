package core

import "akvorado/reporter"

type metrics struct {
	flowsReceived  *reporter.CounterVec
	flowsForwarded *reporter.CounterVec
	flowsErrors    *reporter.CounterVec
}

func (c *Component) initMetrics() {
	c.metrics.flowsReceived = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "flows_received",
			Help: "Number of incoming flows.",
		},
		[]string{"router"},
	)
	c.metrics.flowsForwarded = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "flows_forwarded",
			Help: "Number of flows forwarded to Kafka.",
		},
		[]string{"router"},
	)
	c.metrics.flowsErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "flows_errors",
			Help: "Number of flows with errors.",
		},
		[]string{"router", "error"},
	)
}
