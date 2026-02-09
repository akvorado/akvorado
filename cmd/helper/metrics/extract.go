// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package metrics extracts Prometheus metric definitions from source code.
package metrics

import (
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

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

// Info collects the information about each metric.
type Info struct {
	Name   string   `yaml:"name" json:"name"`
	Type   string   `yaml:"type" json:"type"`
	Help   string   `yaml:"help" json:"help"`
	Labels []string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// Extract walks the AST of all packages and returns sorted, deduplicated metrics.
func Extract(pkgs []*packages.Package) []Info {
	var metrics []Info

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

				m := Info{
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

	slices.SortFunc(metrics, func(a, b Info) int {
		return strings.Compare(a.Name, b.Name)
	})
	metrics = slices.CompactFunc(metrics, func(a, b Info) bool {
		return a.Name == b.Name
	})
	return metrics
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
			name = astStringLiteralValue(kv.Value)
		case "Help":
			help = astStringLiteralValue(kv.Value)
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
