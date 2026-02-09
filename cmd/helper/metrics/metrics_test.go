// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package metrics

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"unicode"

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

	got := Extract(pkgs)
	expected := []Info{
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
		t.Fatalf("Extract() (-got, +want):\n%s", diff)
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

	got := Extract(pkgs)
	expected := []Info{
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
		t.Fatalf("Extract() (-got, +want):\n%s", diff)
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
		Help: "Akvorado build information.",
	}, []string{"version", "compiler"})
	r.GaugeFunc(GaugeOpts{
		Name: "uptime_seconds",
		Help: "Number of seconds the application is running.",
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

	got := Extract(pkgs)
	expected := []Info{
		{
			Name:   "akvorado_cmd_info",
			Type:   "gauge",
			Help:   "Akvorado build information.",
			Labels: []string{"version", "compiler"},
		},
		{
			Name: "akvorado_cmd_uptime_seconds",
			Type: "gauge",
			Help: "Number of seconds the application is running.",
		},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Extract() (-got, +want):\n%s", diff)
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

	got := Extract(pkgs)
	var expected []Info
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Extract() (-got, +want):\n%s", diff)
	}
}

func TestMetricsMarkdown(t *testing.T) {
	metrics := []Info{
		{Name: "akvorado_cmd_info", Type: "gauge", Help: "Akvorado build information."},
		{Name: "akvorado_cmd_uptime_seconds", Type: "gauge", Help: "Number of seconds the application is running."},
		{Name: "akvorado_inlet_flow_active_connections", Type: "gauge", Help: "Number of active connections."},
		{Name: "akvorado_outlet_core_latency_seconds", Type: "summary", Help: "Latency distribution."},
		{Name: "akvorado_outlet_kafka_errors_total", Type: "counter", Help: "Total errors."},
	}

	sections := GroupByPrefix(metrics)
	expected := []Section{
		{
			Prefix: "akvorado_cmd",
			Metrics: []Info{
				{Name: "akvorado_cmd_info", Type: "gauge", Help: "Akvorado build information."},
				{Name: "akvorado_cmd_uptime_seconds", Type: "gauge", Help: "Number of seconds the application is running."},
			},
		},
		{
			Prefix: "akvorado_inlet",
			Metrics: []Info{
				{Name: "akvorado_inlet_flow_active_connections", Type: "gauge", Help: "Number of active connections."},
			},
		},
		{
			Prefix: "akvorado_outlet",
			Metrics: []Info{
				{Name: "akvorado_outlet_core_latency_seconds", Type: "summary", Help: "Latency distribution."},
				{Name: "akvorado_outlet_kafka_errors_total", Type: "counter", Help: "Total errors."},
			},
		},
	}
	if diff := helpers.Diff(sections, expected); diff != "" {
		t.Fatalf("GroupByPrefix() (-got, +want):\n%s", diff)
	}

	var buf strings.Builder
	if err := MarkdownTmpl.Execute(&buf, sections); err != nil {
		t.Fatalf("MarkdownTmpl.Execute() error:\n%+v", err)
	}
	// Only keep the sections and the table from the markdown.
	var filtered []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "|") {
			filtered = append(filtered, line)
		}
	}
	got := strings.Join(filtered, "\n")
	expectedMarkdown := strings.Join([]string{
		"# Metrics",
		"## akvorado_cmd",
		"| Name | Type | Help |",
		"|------|------|------|",
		"| `info` | gauge | Akvorado build information. |",
		"| `uptime\u00ad_seconds` | gauge | Number of seconds the application is running. |",
		"## akvorado_inlet",
		"| Name | Type | Help |",
		"|------|------|------|",
		"| `flow\u00ad_active\u00ad_connections` | gauge | Number of active connections. |",
		"## akvorado_outlet",
		"| Name | Type | Help |",
		"|------|------|------|",
		"| `core\u00ad_latency\u00ad_seconds` | summary | Latency distribution. |",
		"| `kafka\u00ad_errors\u00ad_total` | counter | Total errors. |",
	}, "\n")
	if diff := helpers.Diff(got, expectedMarkdown); diff != "" {
		t.Fatalf("markdown output (-got, +want):\n%s", diff)
	}
}

func TestMetricsHelpStrings(t *testing.T) {
	t.Chdir("../../..")
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedModule,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatalf("packages.Load() error:\n%+v", err)
	}

	metrics := Extract(pkgs)
	for _, m := range metrics {
		if m.Help == "" {
			t.Errorf("%s: help string is empty", m.Name)
			continue
		}
		if !unicode.IsUpper(rune(m.Help[0])) {
			t.Errorf("%s: help string should start with a capital letter: %q", m.Name, m.Help)
		}
		if !unicode.IsPunct(rune(m.Help[len(m.Help)-1])) {
			t.Errorf("%s: help string should end with punctuation: %q", m.Name, m.Help)
		}
	}
}
