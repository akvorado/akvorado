package core

import (
	"akvorado/reporter"
	"sync/atomic"
)

type metrics struct {
	flowsReceived    *reporter.CounterVec
	flowsForwarded   *reporter.CounterVec
	flowsErrors      *reporter.CounterVec
	flowsHTTPClients reporter.GaugeFunc
}

func (c *Component) initMetrics() {
	c.metrics.flowsReceived = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "flows_received",
			Help: "Number of incoming flows.",
		},
		[]string{"sampler"},
	)
	c.metrics.flowsForwarded = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "flows_forwarded",
			Help: "Number of flows forwarded to Kafka.",
		},
		[]string{"sampler"},
	)
	c.metrics.flowsErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "flows_errors",
			Help: "Number of flows with errors.",
		},
		[]string{"sampler", "error"},
	)
	c.metrics.flowsHTTPClients = c.r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "flows_http_clients",
			Help: "Number of HTTP clients requesting flows.",
		},
		func() float64 {
			return float64(atomic.LoadUint32(&c.httpFlowClients))
		},
	)
}
