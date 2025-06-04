// Package ddos implements a simple DDoS detection module.
package ddos

import "akvorado/common/reporter"

type metrics struct {
	detections reporter.Counter
}

func (c *Component) initMetrics() {
	c.metrics.detections = c.r.Counter(
		reporter.CounterOpts{
			Name: "ddos_events_total",
			Help: "Number of detected DDoS events",
		},
	)
}
