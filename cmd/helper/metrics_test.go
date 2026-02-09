// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package main_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	cmd "akvorado/cmd/helper"
	"akvorado/common/helpers"

	"go.yaml.in/yaml/v3"
)

func TestMetricsYAML(t *testing.T) {
	// packages.Load uses ./... which is relative to the working directory.
	t.Chdir("../..")
	root := cmd.RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"metrics"})
	if err := root.Execute(); err != nil {
		t.Fatalf("`metrics` error:\n%+v", err)
	}

	// Parse YAML output
	var out struct {
		Metrics []struct {
			Name string `yaml:"name"`
			Type string `yaml:"type"`
			Help string `yaml:"help"`
		} `yaml:"metrics"`
	}
	if err := yaml.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("Unmarshal() error:\n%+v", err)
	}
	if len(out.Metrics) == 0 {
		t.Fatal("expected at least one metric")
	}

	// Check a known metric
	found := false
	for _, m := range out.Metrics {
		if m.Name == "akvorado_outlet_kafka_received_messages_total" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find akvorado_outlet_kafka_received_messages_total")
	}
}

func TestMetricsJSON(t *testing.T) {
	t.Chdir("../..")
	root := cmd.RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"metrics", "--format", "json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("`metrics --format json` error:\n%+v", err)
	}

	var out struct {
		Metrics []struct {
			Name string `json:"name"`
			Type string `json:"type"`
			Help string `json:"help"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("Unmarshal() error:\n%+v", err)
	}
	if len(out.Metrics) == 0 {
		t.Fatal("expected at least one metric")
	}

	// Check a known metric
	found := false
	for _, m := range out.Metrics {
		if m.Name == "akvorado_outlet_kafka_received_messages_total" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find akvorado_outlet_kafka_received_messages_total")
	}
}

func TestMetricsMarkdownCLI(t *testing.T) {
	t.Chdir("../..")
	root := cmd.RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"metrics", "--format", "markdown"})
	if err := root.Execute(); err != nil {
		t.Fatalf("`metrics --format markdown` error:\n%+v", err)
	}

	var filtered []string
	for line := range strings.SplitSeq(buf.String(), "\n") {
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "|") {
			filtered = append(filtered, line)
		}
	}
	got := filtered[:6]
	expected := []string{
		"# Metrics",
		"## akvorado_cmd",
		"| Name | Type | Help |",
		"|------|------|------|",
		"| `dropped\u00ad_log\u00ad_messages` | gauge | Number of log messages dropped. |",
		"| `info` | gauge | Akvorado build information. |",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("`metrics --format markdown` (-got, +want):\n%s", diff)
	}
}

func TestMetricsInvalidFormat(t *testing.T) {
	root := cmd.RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"metrics", "--format", "xml"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for invalid format")
	}
}
