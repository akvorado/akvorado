// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"akvorado/cmd/helper/metrics"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
	"golang.org/x/tools/go/packages"
)

var metricsFormat string

//go:embed data/metrics.tmpl.md
var markdownTmplStr string

// MarkdownTmpl is the template used for markdown output.
var markdownTmpl = template.Must(template.New("metrics").Funcs(template.FuncMap{
	"formatMetricName": func(name, prefix string) string {
		result := strings.TrimPrefix(name, fmt.Sprintf("%s_", prefix))
		result = strings.ReplaceAll(result, "_", "\u00ad_")
		result = fmt.Sprintf("`%s`", result)
		return result
	},
}).Parse(markdownTmplStr))

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

		m := metrics.Extract(pkgs)

		out := struct {
			Metrics []metrics.Info `yaml:"metrics" json:"metrics"`
		}{Metrics: m}

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
			sections := metrics.GroupByPrefix(m)
			if err := markdownTmpl.Execute(w, sections); err != nil {
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
