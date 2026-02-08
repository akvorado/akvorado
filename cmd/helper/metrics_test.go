// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"akvorado/common/helpers"

	"golang.org/x/tools/go/packages"
)

func TestExtractMetrics(t *testing.T) {
	src := `package fake

type reporter struct{}
type CounterOpts struct {
	Name string
	Help string
}
type GaugeOpts struct {
	Name string
	Help string
}
type HistogramOpts struct {
	Name string
	Help string
}

func (r *reporter) Counter(opts CounterOpts) {}
func (r *reporter) CounterVec(opts CounterOpts, labels []string) {}
func (r *reporter) GaugeFunc(opts GaugeOpts, f func() float64) {}
func (r *reporter) HistogramVec(opts HistogramOpts, labels []string) {}

var r reporter

func init() {
	r.Counter(CounterOpts{
		Name: "requests_total",
		Help: "Total number of requests.",
	})
	r.CounterVec(CounterOpts{
		Name: "errors_total",
		Help: "Total errors.",
	}, []string{"code", "method"})
	r.GaugeFunc(GaugeOpts{
		Name: "temperature",
		Help: "Current temperature.",
	}, func() float64 { return 0 })
	r.HistogramVec(HistogramOpts{
		Name: "duration_seconds",
		Help: "Request duration.",
	}, []string{"handler"})
	// Duplicate: should be deduplicated
	r.Counter(CounterOpts{
		Name: "requests_total",
		Help: "Total number of requests.",
	})
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "fake.go", src, 0)
	if err != nil {
		t.Fatalf("ParseFile() error:\n%+v", err)
	}

	pkgs := []*packages.Package{
		{
			PkgPath: "akvorado/outlet/kafka",
			Name:    "kafka",
			Syntax:  []*ast.File{f},
		},
	}

	got := extractMetrics(pkgs)
	expected := []metricInfo{
		{
			Name:   "akvorado_outlet_kafka_duration_seconds",
			Type:   "histogram",
			Help:   "Request duration.",
			Labels: []string{"handler"},
		},
		{
			Name:   "akvorado_outlet_kafka_errors_total",
			Type:   "counter",
			Help:   "Total errors.",
			Labels: []string{"code", "method"},
		},
		{
			Name: "akvorado_outlet_kafka_requests_total",
			Type: "counter",
			Help: "Total number of requests.",
		},
		{
			Name: "akvorado_outlet_kafka_temperature",
			Type: "gauge",
			Help: "Current temperature.",
		},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("extractMetrics() (-got, +want):\n%s", diff)
	}
}

func TestExtractMetricsMultiplePackages(t *testing.T) {
	src1 := `package pkg1

type r struct{}
type GaugeOpts struct {
	Name string
	Help string
}
func (x *r) Gauge(opts GaugeOpts) {}

var x r
func init() {
	x.Gauge(GaugeOpts{
		Name: "active_connections",
		Help: "Number of active connections.",
	})
}
`
	src2 := `package pkg2

type r struct{}
type SummaryOpts struct {
	Name string
	Help string
}
func (x *r) SummaryVec(opts SummaryOpts, labels []string) {}

var x r
func init() {
	x.SummaryVec(SummaryOpts{
		Name: "latency_seconds",
		Help: "Latency distribution.",
	}, []string{"endpoint"})
}
`
	fset := token.NewFileSet()
	f1, err := parser.ParseFile(fset, "pkg1.go", src1, 0)
	if err != nil {
		t.Fatalf("ParseFile() error:\n%+v", err)
	}
	f2, err := parser.ParseFile(fset, "pkg2.go", src2, 0)
	if err != nil {
		t.Fatalf("ParseFile() error:\n%+v", err)
	}

	pkgs := []*packages.Package{
		{PkgPath: "akvorado/inlet/flow", Name: "flow", Syntax: []*ast.File{f1}},
		{PkgPath: "akvorado/outlet/core", Name: "core", Syntax: []*ast.File{f2}},
	}

	got := extractMetrics(pkgs)
	expected := []metricInfo{
		{
			Name: "akvorado_inlet_flow_active_connections",
			Type: "gauge",
			Help: "Number of active connections.",
		},
		{
			Name:   "akvorado_outlet_core_latency_seconds",
			Type:   "summary",
			Help:   "Latency distribution.",
			Labels: []string{"endpoint"},
		},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("extractMetrics() (-got, +want):\n%s", diff)
	}
}

func TestExtractMetricsPackageMain(t *testing.T) {
	src := `package main

type reporter struct{}
type GaugeOpts struct {
	Name string
	Help string
}
func (r *reporter) GaugeVec(opts GaugeOpts, labels []string) interface{ WithLabelValues(...string) interface{ Set(float64) } } { return nil }
func (r *reporter) GaugeFunc(opts GaugeOpts, f func() float64) {}

var r reporter

func init() {
	r.GaugeVec(GaugeOpts{
		Name: "info",
		Help: "Akvorado build information",
	}, []string{"version", "compiler"})
	r.GaugeFunc(GaugeOpts{
		Name: "uptime_seconds",
		Help: "number of seconds the application is running",
	}, func() float64 { return 0 })
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "root.go", src, 0)
	if err != nil {
		t.Fatalf("ParseFile() error:\n%+v", err)
	}

	pkgs := []*packages.Package{
		{
			PkgPath: "akvorado/cmd/akvorado",
			Name:    "main",
			Module:  &packages.Module{Path: "akvorado"},
			Syntax:  []*ast.File{f},
		},
	}

	got := extractMetrics(pkgs)
	expected := []metricInfo{
		{
			Name:   "akvorado_info",
			Type:   "gauge",
			Help:   "Akvorado build information",
			Labels: []string{"version", "compiler"},
		},
		{
			Name: "akvorado_uptime_seconds",
			Type: "gauge",
			Help: "number of seconds the application is running",
		},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("extractMetrics() (-got, +want):\n%s", diff)
	}
}

func TestExtractMetricsNoMatch(t *testing.T) {
	src := `package fake

type client struct{}
func (c *client) Get(url string) {}
var c client
func init() {
	c.Get("http://example.com")
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "fake.go", src, 0)
	if err != nil {
		t.Fatalf("ParseFile() error:\n%+v", err)
	}

	pkgs := []*packages.Package{
		{PkgPath: "akvorado/console", Name: "console", Syntax: []*ast.File{f}},
	}

	got := extractMetrics(pkgs)
	var expected []metricInfo
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("extractMetrics() (-got, +want):\n%s", diff)
	}
}
