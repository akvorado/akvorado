package bioris

import "akvorado/common/reporter"

type metrics struct {
	risUp                    *reporter.GaugeVec
	knownRouters             *reporter.CounterVec
	lpmRequests              *reporter.CounterVec
	lpmRequestErrors         *reporter.CounterVec
	lpmRequestTimeouts       *reporter.CounterVec
	lpmRequestSuccess        *reporter.CounterVec
	routerChosenFallback     *reporter.CounterVec
	routerChosenAgentIDMatch *reporter.CounterVec
}

// initMetrics initialize the metrics for the BMP component.
func (c *Provider) initMetrics() {
	// if this is used in testing, we don't have client metrics, so check this
	if c.clientMetrics != nil {
		c.r.MetricCollector(c.clientMetrics)
	}
	c.metrics.risUp = c.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "connection_up",
			Help: "Connection to BioRIS instance up.",
		},
		[]string{"ris"},
	)
	c.metrics.knownRouters = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "known_routers_total",
			Help: "Number of known routers per RIS.",
		},
		[]string{"ris"},
	)
	c.metrics.lpmRequests = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "lpm_requests_total",
			Help: "Number of LPM requests per RIS and router.",
		},
		[]string{"ris", "router"},
	)
	c.metrics.lpmRequestErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "lpm_request_errors",
			Help: "Number of failed LPM requests per RIS and router.",
		},
		[]string{"ris", "router"},
	)
	c.metrics.lpmRequestTimeouts = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "lpm_request_timeouts",
			Help: "Timed out LPM requests per RIS and router.",
		},
		[]string{"ris", "router"},
	)
	c.metrics.lpmRequestSuccess = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "lpm_request_success",
			Help: "Number of successfull requests per RIS and router.",
		},
		[]string{"ris", "router"},
	)
	c.metrics.routerChosenAgentIDMatch = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "router_request_agentid",
			Help: "Number of times the router/ris combination was returned with an exact match of the agent ID.",
		},
		[]string{"ris", "router"},
	)
	c.metrics.routerChosenFallback = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "router_request_fallback",
			Help: "Number of times the router/ris combination was returned without an exact match of the agent ID.",
		},
		[]string{"ris", "router"},
	)
}
