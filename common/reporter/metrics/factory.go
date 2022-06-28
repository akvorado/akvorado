// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// Factory allow registration of new metrics and returns existing
// metrics if they were already registered.
type Factory struct {
	prefix   string
	registry *prometheus.Registry
}

func (f *Factory) prefixWith(name string) string {
	return fmt.Sprintf("%s%s", f.prefix, name)
}

// NewCounter works like the function of the same name in the prometheus package
// but it automatically registers the Counter with the Factory's Registerer.
func (f *Factory) NewCounter(opts prometheus.CounterOpts) prometheus.Counter {
	opts.Name = f.prefixWith(opts.Name)
	c := prometheus.NewCounter(opts)
	if err := f.registry.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.Counter)
		}
		panic(err)
	}
	return c
}

// NewCounterVec works like the function of the same name in the prometheus
// package but it automatically registers the CounterVec with the Factory's
// Registerer.
func (f *Factory) NewCounterVec(opts prometheus.CounterOpts, labelNames []string) *prometheus.CounterVec {
	opts.Name = f.prefixWith(opts.Name)
	c := prometheus.NewCounterVec(opts, labelNames)
	if err := f.registry.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(*prometheus.CounterVec)
		}
		panic(err)
	}
	return c
}

// NewCounterFunc works like the function of the same name in the prometheus
// package but it automatically registers the CounterFunc with the Factory's
// Registerer.
func (f *Factory) NewCounterFunc(opts prometheus.CounterOpts, function func() float64) prometheus.CounterFunc {
	opts.Name = f.prefixWith(opts.Name)
	c := prometheus.NewCounterFunc(opts, function)
	if err := f.registry.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.CounterFunc)
		}
		panic(err)
	}
	return c
}

// NewGauge works like the function of the same name in the prometheus package
// but it automatically registers the Gauge with the Factory's Registerer.
func (f *Factory) NewGauge(opts prometheus.GaugeOpts) prometheus.Gauge {
	opts.Name = f.prefixWith(opts.Name)
	c := prometheus.NewGauge(opts)
	if err := f.registry.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.Gauge)
		}
		panic(err)
	}
	return c
}

// NewGaugeVec works like the function of the same name in the prometheus
// package but it automatically registers the GaugeVec with the Factory's
// Registerer.
func (f *Factory) NewGaugeVec(opts prometheus.GaugeOpts, labelNames []string) *prometheus.GaugeVec {
	opts.Name = f.prefixWith(opts.Name)
	c := prometheus.NewGaugeVec(opts, labelNames)
	if err := f.registry.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(*prometheus.GaugeVec)
		}
		panic(err)
	}
	return c
}

// NewGaugeFunc works like the function of the same name in the prometheus
// package but it automatically registers the GaugeFunc with the Factory's
// Registerer.
func (f *Factory) NewGaugeFunc(opts prometheus.GaugeOpts, function func() float64) prometheus.GaugeFunc {
	opts.Name = f.prefixWith(opts.Name)
	c := prometheus.NewGaugeFunc(opts, function)
	if err := f.registry.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.GaugeFunc)
		}
		panic(err)
	}
	return c
}

// NewSummary works like the function of the same name in the prometheus package
// but it automatically registers the Summary with the Factory's Registerer.
func (f *Factory) NewSummary(opts prometheus.SummaryOpts) prometheus.Summary {
	opts.Name = f.prefixWith(opts.Name)
	c := prometheus.NewSummary(opts)
	if err := f.registry.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.Summary)
		}
		panic(err)
	}
	return c
}

// NewSummaryVec works like the function of the same name in the prometheus
// package but it automatically registers the SummaryVec with the Factory's
// Registerer.
func (f *Factory) NewSummaryVec(opts prometheus.SummaryOpts, labelNames []string) *prometheus.SummaryVec {
	opts.Name = f.prefixWith(opts.Name)
	c := prometheus.NewSummaryVec(opts, labelNames)
	if err := f.registry.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(*prometheus.SummaryVec)
		}
		panic(err)
	}
	return c
}

// NewHistogram works like the function of the same name in the prometheus
// package but it automatically registers the Histogram with the Factory's
// Registerer.
func (f *Factory) NewHistogram(opts prometheus.HistogramOpts) prometheus.Histogram {
	opts.Name = f.prefixWith(opts.Name)
	c := prometheus.NewHistogram(opts)
	if err := f.registry.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(prometheus.Histogram)
		}
		panic(err)
	}
	return c
}

// NewHistogramVec works like the function of the same name in the prometheus
// package but it automatically registers the HistogramVec with the Factory's
// Registerer.
func (f *Factory) NewHistogramVec(opts prometheus.HistogramOpts, labelNames []string) *prometheus.HistogramVec {
	opts.Name = f.prefixWith(opts.Name)
	c := prometheus.NewHistogramVec(opts, labelNames)
	if err := f.registry.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(*prometheus.HistogramVec)
		}
		panic(err)
	}
	return c
}
