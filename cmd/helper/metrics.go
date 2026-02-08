// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"

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
		default:
			return fmt.Errorf("unknown format %q (expected yaml or json)", metricsFormat)
		}
	},
}

func init() {
	metricsCmd.Flags().StringVarP(&metricsFormat, "format", "f", "yaml", "Output format (yaml or json)")
	RootCmd.AddCommand(metricsCmd)
}

// methodTypes maps method names to Prometheus metric types.
var methodTypes = map[string]string{
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

// extractMetrics walks the AST of all packages and returns sorted, deduplicated metrics.
func extractMetrics(pkgs []*packages.Package) []metricInfo {
	var metrics []metricInfo

	for _, pkg := range pkgs {
		var modulePath string
		if pkg.Module != nil {
			modulePath = pkg.Module.Path
		}
		prefix := computePrefix(pkg.PkgPath, pkg.Name, modulePath)
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
				metricType, ok := methodTypes[methodName]
				if !ok {
					return true
				}
				if len(call.Args) < 1 {
					return true
				}

				name, help := extractOpts(call.Args[0])
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
					m.Labels = extractLabels(call.Args[1])
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

// computePrefix replicates the prefix logic from common/reporter/metrics/root.go.
// At runtime, getPrefix receives a function name from the call stack. For "package main"
// binaries, Go uses "main.funcName" which doesn't start with the module name, so the
// prefix falls back to moduleName + "/cmd" (e.g. "akvorado_cmd_").
func computePrefix(pkgPath, pkgName, modulePath string) string {
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

// extractOpts extracts Name and Help from a composite literal (the opts argument).
func extractOpts(expr ast.Expr) (name, help string) {
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
			name = stringLiteralValue(kv.Value)
		case "Help":
			help = stringLiteralValue(kv.Value)
		}
	}
	return name, help
}

// extractLabels extracts label names from a []string{...} composite literal.
func extractLabels(expr ast.Expr) []string {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	var labels []string
	for _, elt := range lit.Elts {
		if v := stringLiteralValue(elt); v != "" {
			labels = append(labels, v)
		}
	}
	return labels
}

// stringLiteralValue returns the unquoted value of a string literal, or "".
func stringLiteralValue(expr ast.Expr) string {
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
