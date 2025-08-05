// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"strings"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestDemoExporterStart(t *testing.T) {
	r := reporter.NewMock(t)
	config := DemoExporterConfiguration{}
	config.Reset()
	if err := demoExporterStart(r, config, true); err != nil {
		t.Fatalf("demoExporterStart() error:\n%+v", err)
	}
}

func TestDemoExporter(t *testing.T) {
	root := RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"demo-exporter", "--check", "/dev/null"})
	t.Setenv("AKVORADO_CFG_DEMOEXPORTER_SNMP_NAME", "test")
	err := root.Execute()
	if err == nil {
		t.Fatal("`demo-exporter` should produce an error")
	}

	want := []string{
		`invalid configuration:`,
		`Key: 'DemoExporterConfiguration.SNMP.Interfaces' Error:Field validation for 'Interfaces' failed on the 'min' tag`,
		`Key: 'DemoExporterConfiguration.Flows.Flows' Error:Field validation for 'Flows' failed on the 'min' tag`,
		`Key: 'DemoExporterConfiguration.Flows.Target' Error:Field validation for 'Target' failed on the 'required' tag`,
	}
	got := strings.Split(err.Error(), "\n")
	if diff := helpers.Diff(got, want); diff != "" {
		t.Fatalf("`demo-exporter` (-got, +want):\n%s", diff)
	}
}
