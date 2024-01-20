// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package gnmi

import "akvorado/common/reporter"

// metrics is the set of metrics for the provider.
type metrics struct {
	collectorCount reporter.Counter
	ready          *reporter.GaugeVec
	models         *reporter.GaugeVec
	encodings      *reporter.GaugeVec
	errors         *reporter.CounterVec
	updates        *reporter.CounterVec
	paths          *reporter.GaugeVec
	times          *reporter.SummaryVec
}

// initMetrics initialize metrics for the provider.
func (p *Provider) initMetrics() {
	p.metrics.collectorCount = p.r.Counter(
		reporter.CounterOpts{
			Name: "collector_count",
			Help: "Number of collectors running.",
		},
	)
	p.metrics.ready = p.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "collector_ready_info",
			Help: "Is the collector ready?",
		},
		[]string{"exporter"},
	)
	p.metrics.models = p.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "model_info",
			Help: "Model used for an exporter.",
		},
		[]string{"exporter", "model"},
	)
	p.metrics.encodings = p.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "encoding_info",
			Help: "Encoding used for an exporter.",
		},
		[]string{"exporter", "encoding"},
	)
	p.metrics.errors = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Errors reported for an exporter.",
		},
		[]string{"exporter", "error"},
	)
	p.metrics.updates = p.r.CounterVec(
		reporter.CounterOpts{
			Name: "updates_total",
			Help: "Number of updates for an exporter.",
		},
		[]string{"exporter"},
	)
	p.metrics.paths = p.r.GaugeVec(
		reporter.GaugeOpts{
			Name: "paths_count",
			Help: "Number paths collected from an exporter.",
		},
		[]string{"exporter"},
	)
	p.metrics.times = p.r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "collector_seconds",
			Help:       "Time to successfully fetch values from an exporter.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}, []string{"exporter"},
	)
}
