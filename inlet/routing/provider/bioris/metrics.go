package bioris

import "akvorado/common/reporter"

type metrics struct {
	risUp                    *reporter.GaugeVec
	knownRouters             *reporter.GaugeVec
	lpmRequests              *reporter.CounterVec
	lpmRequestErrors         *reporter.CounterVec
	lpmRequestTimeouts       *reporter.CounterVec
	lpmRequestSuccess        *reporter.CounterVec
	routerChosenFallback     *reporter.CounterVec
	routerChosenAgentIDMatch *reporter.CounterVec
}

// initMetrics initialize the metrics for the BMP component.
func (p *Provider) initMetrics() {
	p.r.MetricCollector(p.clientMetrics)
	p.metrics.risUp = p.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "connection_up",
			Help: "Connection to BioRIS instance up.",
		},
		[]string{"ris"},
	)
	p.metrics.knownRouters = p.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "known_routers_total",
			Help: "Number of known routers per RIS.",
		},
		[]string{"ris"},
	)
	p.metrics.lpmRequests = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "lpm_requests_total",
			Help: "Number of LPM requests per RIS and router.",
		},
		[]string{"ris", "router"},
	)
	p.metrics.lpmRequestErrors = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "lpm_request_errors_total",
			Help: "Number of failed LPM requests per RIS and router.",
		},
		[]string{"ris", "router"},
	)
	p.metrics.lpmRequestTimeouts = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "lpm_request_timeouts_total",
			Help: "Timed out LPM requests per RIS and router.",
		},
		[]string{"ris", "router"},
	)
	p.metrics.lpmRequestSuccess = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "lpm_success_requests_total",
			Help: "Number of successfull requests per RIS and router.",
		},
		[]string{"ris", "router"},
	)
	p.metrics.routerChosenAgentIDMatch = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "router_agentid_requests_total",
			Help: "Number of times the router/ris combination was returned with an exact match of the agent ID.",
		},
		[]string{"ris", "router"},
	)
	p.metrics.routerChosenFallback = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "router_fallback_requests_total",
			Help: "Number of times the router/ris combination was returned without an exact match of the agent ID.",
		},
		[]string{"ris", "router"},
	)
}
