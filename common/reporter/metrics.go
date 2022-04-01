// Metrics façade for reporter.
//
// It supports all methods from a factory (except UntypedFunc). See
// https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/promauto#Factory.
// Unlike promauto, it will accepts duplicate registration.

package reporter

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

// Register some aliases to avoid importing prometheus package.

type (
	// CounterOpts defines options for counters
	CounterOpts = prometheus.CounterOpts
	// GaugeOpts defines options for gauges
	GaugeOpts = prometheus.GaugeOpts
	// HistogramOpts defines options for histograms
	HistogramOpts = prometheus.HistogramOpts
	// SummaryOpts defines options for summaries
	SummaryOpts = prometheus.SummaryOpts
	// UntypedOpts defines options for untypeds
	UntypedOpts = prometheus.UntypedOpts

	// Counter defines counters
	Counter = prometheus.Counter
	// CounterFunc defines counter functions
	CounterFunc = prometheus.CounterFunc
	// CounterVec defines counter vectors
	CounterVec = prometheus.CounterVec
	// Gauge defines gauges
	Gauge = prometheus.Gauge
	// GaugeFunc defines gauge functions
	GaugeFunc = prometheus.GaugeFunc
	// GaugeVec defines gauge vectors
	GaugeVec = prometheus.GaugeVec
	// Histogram defines histograms
	Histogram = prometheus.Histogram
	// HistogramVec defines histogram vectors
	HistogramVec = prometheus.HistogramVec
	// Summary defines summarys
	Summary = prometheus.Summary
	// SummaryVec defines summary vectors
	SummaryVec = prometheus.SummaryVec
	// UntypedFunc defines untyped functions
	UntypedFunc = prometheus.UntypedFunc

	// MetricDesc defines a metric description
	MetricDesc = prometheus.Desc
)

// Counter mimics NewCounter from promauto package.
func (r *Reporter) Counter(opts CounterOpts) Counter {
	return r.metrics.Factory(1).NewCounter(opts)
}

// CounterFunc mimics NewCounterFunc from promauto package.
func (r *Reporter) CounterFunc(opts CounterOpts, function func() float64) CounterFunc {
	return r.metrics.Factory(1).NewCounterFunc(opts, function)
}

// CounterVec mimics NewCounterVec from promauto package.
func (r *Reporter) CounterVec(opts CounterOpts, labelNames []string) *CounterVec {
	return r.metrics.Factory(1).NewCounterVec(opts, labelNames)
}

// Gauge mimics NewGauge from promauto package.
func (r *Reporter) Gauge(opts GaugeOpts) Gauge {
	return r.metrics.Factory(1).NewGauge(opts)
}

// GaugeFunc mimics NewGaugeFunc from promauto package.
func (r *Reporter) GaugeFunc(opts GaugeOpts, function func() float64) GaugeFunc {
	return r.metrics.Factory(1).NewGaugeFunc(opts, function)
}

// GaugeVec mimics NewGaugeVec from promauto package.
func (r *Reporter) GaugeVec(opts GaugeOpts, labelNames []string) *GaugeVec {
	return r.metrics.Factory(1).NewGaugeVec(opts, labelNames)
}

// Histogram mimics NewHistogram from promauto package.
func (r *Reporter) Histogram(opts HistogramOpts) Histogram {
	return r.metrics.Factory(1).NewHistogram(opts)
}

// HistogramVec mimics NewHistogramVec from promauto package.
func (r *Reporter) HistogramVec(opts HistogramOpts, labelNames []string) *HistogramVec {
	return r.metrics.Factory(1).NewHistogramVec(opts, labelNames)
}

// Summary mimics NewSummary from promauto package.
func (r *Reporter) Summary(opts SummaryOpts) Summary {
	return r.metrics.Factory(1).NewSummary(opts)
}

// SummaryVec mimics NewSummaryVec from promauto package.
func (r *Reporter) SummaryVec(opts SummaryOpts, labelNames []string) *SummaryVec {
	return r.metrics.Factory(1).NewSummaryVec(opts, labelNames)
}

// MetricsHTTPHandler returns the HTTP handler to get metrics.
func (r *Reporter) MetricsHTTPHandler() http.Handler {
	return r.metrics.HTTPHandler()
}

// MetricCollector register a custom collector.
func (r *Reporter) MetricCollector(c prometheus.Collector) {
	r.metrics.Collector(c)
}

// MetricDesc defines a new metric description.
func (r *Reporter) MetricDesc(name, help string, variableLabels []string) *MetricDesc {
	return r.metrics.Desc(1, name, help, variableLabels)
}
