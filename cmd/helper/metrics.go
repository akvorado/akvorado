// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
	"golang.org/x/tools/go/packages"
)

var metricsFormat string

// metricsCmd extracts Prometheus metric definitions from the codebase.
var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Extract Prometheus metric definitions from source code",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg := &packages.Config{
			Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedModule,
		}
		pkgs, err := packages.Load(cfg, "./...")
		if err != nil {
			return fmt.Errorf("unable to load packages: %w", err)
		}

		metrics := extractMetrics(pkgs)

		out := struct {
			Metrics []metricInfo `yaml:"metrics" json:"metrics"`
		}{Metrics: metrics}

		w := cmd.OutOrStdout()
		switch metricsFormat {
		case "yaml":
			enc := yaml.NewEncoder(w)
			enc.SetIndent(2)
			if err := enc.Encode(out); err != nil {
				return fmt.Errorf("unable to encode YAML: %w", err)
			}
			return enc.Close()
		case "json":
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			if err := enc.Encode(out); err != nil {
				return fmt.Errorf("unable to encode JSON: %w", err)
			}
			return nil
		case "markdown":
			sections := groupMetricsByPrefix(metrics)
			if err := metricMarkdownTmpl.Execute(w, sections); err != nil {
				return fmt.Errorf("unable to render markdown: %w", err)
			}
			return nil
		default:
			return fmt.Errorf("unknown format %q (expected yaml, json or markdown)", metricsFormat)
		}
	},
}

func init() {
	metricsCmd.Flags().StringVarP(&metricsFormat, "format", "f", "yaml", "Output format (yaml, json or markdown)")
	RootCmd.AddCommand(metricsCmd)
}

// metricMethodTypes maps method names to Prometheus metric types.
var metricMethodTypes = map[string]string{
	"Counter":      "counter",
	"CounterVec":   "counter",
	"CounterFunc":  "counter",
	"Gauge":        "gauge",
	"GaugeVec":     "gauge",
	"GaugeFunc":    "gauge",
	"Histogram":    "histogram",
	"HistogramVec": "histogram",
	"Summary":      "summary",
	"SummaryVec":   "summary",
}

// metricInfo collects the information about each metric.
type metricInfo struct {
	Name   string   `yaml:"name" json:"name"`
	Type   string   `yaml:"type" json:"type"`
	Help   string   `yaml:"help" json:"help"`
	Labels []string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// metricSection groups metrics under a common top-level prefix.
type metricSection struct {
	Prefix  string
	Metrics []metricInfo
}

//go:embed data/metrics.tmpl.md
var metricMarkdownTmplStr string
var metricMarkdownTmpl = template.Must(template.New("metrics").Funcs(template.FuncMap{
	"formatMetricName": func(name, prefix string) string {
		result := strings.TrimPrefix(name, fmt.Sprintf("%s_", prefix))
		result = strings.ReplaceAll(result, "_", "\u00ad_")
		result = fmt.Sprintf("`%s`", result)
		return result
	},
}).Parse(metricMarkdownTmplStr))

// extractMetrics walks the AST of all packages and returns sorted, deduplicated metrics.
func extractMetrics(pkgs []*packages.Package) []metricInfo {
	var metrics []metricInfo

	for _, pkg := range pkgs {
		var modulePath string
		if pkg.Module != nil {
			modulePath = pkg.Module.Path
		}
		prefix := computeMetricPrefix(pkg.PkgPath, pkg.Name, modulePath)
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				methodName := sel.Sel.Name
				metricType, ok := metricMethodTypes[methodName]
				if !ok {
					return true
				}
				if len(call.Args) < 1 {
					return true
				}

				name, help := extractMetricOpts(call.Args[0])
				if name == "" {
					return true
				}

				m := metricInfo{
					Name: prefix + name,
					Type: metricType,
					Help: help,
				}

				// Extract labels for Vec variants
				if strings.HasSuffix(methodName, "Vec") && len(call.Args) >= 2 {
					m.Labels = extractMetricLabels(call.Args[1])
				}

				metrics = append(metrics, m)
				return true
			})
		}
	}

	slices.SortFunc(metrics, func(a, b metricInfo) int {
		return strings.Compare(a.Name, b.Name)
	})
	metrics = slices.CompactFunc(metrics, func(a, b metricInfo) bool {
		return a.Name == b.Name
	})
	return metrics
}

// computeMetricPrefix replicates the prefix logic from common/reporter/metrics/root.go.
// At runtime, getPrefix receives a function name from the call stack. For "package main"
// binaries, Go uses "main.funcName" which doesn't start with the module name, so the
// prefix falls back to moduleName + "/cmd" (e.g. "akvorado_cmd_").
func computeMetricPrefix(pkgPath, pkgName, modulePath string) string {
	var name string
	if pkgName == "main" {
		name = modulePath + "/cmd"
	} else {
		name = pkgPath
	}
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, ".", "_")
	return name + "_"
}

// extractMetricOpts extracts Name and Help from a composite literal (the opts argument).
func extractMetricOpts(expr ast.Expr) (name, help string) {
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		expr = unary.X
	}
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return "", ""
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Name":
			name = astStringLiteralValue(kv.Value)
		case "Help":
			help = astStringLiteralValue(kv.Value)
		}
	}
	return name, help
}

// extractMetricLabels extracts label names from a []string{...} composite literal.
func extractMetricLabels(expr ast.Expr) []string {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	var labels []string
	for _, elt := range lit.Elts {
		if v := astStringLiteralValue(elt); v != "" {
			labels = append(labels, v)
		}
	}
	return labels
}

// astStringLiteralValue returns the unquoted value of a string literal, or "".
func astStringLiteralValue(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return s
}

// groupMetricsByPrefix groups sorted metrics by their top-level prefix
// (the first two underscore-separated components, e.g. "akvorado_outlet").
func groupMetricsByPrefix(metrics []metricInfo) []metricSection {
	var sections []metricSection
	for _, m := range metrics {
		prefix := metricTopLevelPrefix(m.Name)
		if len(sections) == 0 || sections[len(sections)-1].Prefix != prefix {
			sections = append(sections, metricSection{Prefix: prefix})
		}
		sections[len(sections)-1].Metrics = append(sections[len(sections)-1].Metrics, m)
	}
	return sections
}

// metricTopLevelPrefix returns the first two underscore-separated components of name.
func metricTopLevelPrefix(name string) string {
	idx := strings.IndexByte(name, '_')
	if idx < 0 {
		return name
	}
	idx2 := strings.IndexByte(name[idx+1:], '_')
	if idx2 < 0 {
		return name
	}
	return name[:idx+1+idx2]
}
