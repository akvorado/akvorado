package bioris

import "akvorado/common/reporter"

type metrics struct {
	risUp                     *reporter.GaugeVec
	knownRouters              *reporter.CounterVec
	lpmRequests               *reporter.CounterVec
	lpmRequestErrors          *reporter.CounterVec
	lpmRequestContextCanceled *reporter.CounterVec
	lpmRequestSuccess         *reporter.CounterVec
	routerChosenRandom        *reporter.CounterVec
	routerChosenAgentIDMatch  *reporter.CounterVec
}

// initMetrics initialize the metrics for the BMP component.
func (c *Component) initMetrics() {
	// if this is used in testing, we don't have client metrics, so check this
	if c.clientMetrics != nil {
		c.r.MetricCollector(c.clientMetrics)
	}
	c.metrics.risUp = c.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "connection_up",
			Help: "Connection to BioRIS Instance Up",
		},
		[]string{"ris"},
	)
	c.metrics.knownRouters = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "known_routers",
			Help: "Number of known routers per RIS",
		},
		[]string{"ris"},
	)
	c.metrics.lpmRequests = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "lpm_requests",
			Help: "Number of lpm requests per RIS and Router",
		},
		[]string{"ris", "router"},
	)
	c.metrics.lpmRequestErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "lpm_request_errors",
			Help: "Number of failed lpm requests per RIS and Router",
		},
		[]string{"ris", "router"},
	)
	c.metrics.lpmRequestContextCanceled = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "lpm_request_context_canceled",
			Help: "Timed out lpm requests per RIS and Router",
		},
		[]string{"ris", "router"},
	)
	c.metrics.lpmRequestSuccess = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "lpm_request_success",
			Help: "Number of successfull requests per RIS and Router",
		},
		[]string{"ris", "router"},
	)
	c.metrics.routerChosenAgentIDMatch = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "router_chosen_agent_id_match",
			Help: "Numbers the router was chosen because the agent id matched the router id",
		},
		[]string{"ris", "router"},
	)
	c.metrics.routerChosenRandom = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "router_chosen_random",
			Help: "Numbers the router was chosen randomly",
		},
		[]string{"ris", "router"},
	)
}
