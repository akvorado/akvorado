// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/demoexporter/flows"
	"akvorado/demoexporter/snmp"
)

func TestDemoExporterStart(t *testing.T) {
	r := reporter.NewMock(t)
	config := DemoExporterConfiguration{}
	config.Reset()
	// The configuration describes the fleet and how the traffic is
	// spread over it.
	config.DemoExporter.Hostname = "exporter1"
	config.Flows.Target = "127.0.0.1:2055"
	config.Flows.Exporters = map[string]flows.ExporterConfiguration{
		"exporter1": {
			SamplingRate: 1000,
			Interfaces: map[int]flows.InterfaceConfiguration{
				10: {
					InterfaceConfiguration: snmp.InterfaceConfiguration{Description: "Transit: Telia"},
					Boundary:               schema.InterfaceBoundaryExternal,
					ASN:                    1299,
				},
				20: {
					InterfaceConfiguration: snmp.InterfaceConfiguration{Description: "Core"},
					Boundary:               schema.InterfaceBoundaryInternal,
				},
			},
		},
	}
	config.Flows.Mapping = []flows.MappingRuleConfiguration{
		{
			Assign: []flows.AssignConfiguration{
				{Exporter: "exporter1", Interface: 10, Weight: 1},
			},
		},
	}
	if err := demoExporterStart(r, config, true); err != nil {
		t.Fatalf("demoExporterStart() error:\n%+v", err)
	}
}

func TestDemoExporterShippedConfiguration(t *testing.T) {
	for _, name := range []string{"demo-1.yaml", "demo-2.yaml", "demo-3.yaml", "demo-4.yaml"} {
		config := DemoExporterConfiguration{}
		options := ConfigRelatedOptions{Path: filepath.Join("../../config", name)}
		if _, err := options.Parse(io.Discard, "demo-exporter", &config); err != nil {
			t.Fatalf("Parse(%q) error:\n%+v", name, err)
		}
		if err := demoExporterStart(reporter.NewMock(t), config, true); err != nil {
			t.Fatalf("demoExporterStart(%q) error:\n%+v", name, err)
		}
	}
}

func TestDemoExporter(t *testing.T) {
	root := RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"demo-exporter", "--check", "/dev/null"})
	err := root.Execute()
	if err == nil {
		t.Fatal("`demo-exporter` should produce an error")
	}

	want := []string{
		`invalid configuration:`,
		`Key: 'DemoExporterConfiguration.DemoExporter.Hostname' Error:Field validation for 'Hostname' failed on the 'required' tag`,
		`Key: 'DemoExporterConfiguration.Flows.Target' Error:Field validation for 'Target' failed on the 'required' tag`,
		`Key: 'DemoExporterConfiguration.Flows.Exporters' Error:Field validation for 'Exporters' failed on the 'min' tag`,
		`Key: 'DemoExporterConfiguration.Flows.Mapping' Error:Field validation for 'Mapping' failed on the 'min' tag`,
	}
	got := strings.Split(err.Error(), "\n")
	if diff := helpers.Diff(got, want); diff != "" {
		t.Fatalf("`demo-exporter` (-got, +want):\n%s", diff)
	}
}
