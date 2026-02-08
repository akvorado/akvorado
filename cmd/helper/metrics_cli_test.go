// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package main_test

import (
	"bytes"
	"encoding/json"
	"testing"

	cmd "akvorado/cmd/helper"

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
		t.Fatalf("YAML parse error:\n%+v", err)
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
		t.Fatalf("JSON parse error:\n%+v", err)
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

func TestMetricsInvalidFormat(t *testing.T) {
	root := cmd.RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"metrics", "--format", "xml"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for invalid format")
	}
}
