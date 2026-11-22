// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDemoExporterInspect(t *testing.T) {
	root := RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"demo-exporter", "inspect"})
	if err := root.Execute(); err != nil {
		t.Fatalf("`demo-exporter inspect` error:\n%+v", err)
	}
	for _, expected := range []string{
		"Model: ", "Local prefixes: ", "Remote prefixes: ",
		"Protocols: ", "Ports: ", "Rate profile: ", "Speed: ", "Flows:\n  ",
	} {
		if !strings.Contains(buf.String(), expected) {
			t.Errorf("`demo-exporter inspect` output misses %q:\n%s", expected, buf.String())
		}
	}
}

func TestDemoExporterInspectMissing(t *testing.T) {
	root := RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"demo-exporter", "inspect", "/nonexistent.model"})
	if err := root.Execute(); err == nil {
		t.Fatal("`demo-exporter inspect` should produce an error")
	}
}

func TestASNNames(t *testing.T) {
	got := asnNames([]uint32{15169, 64496})
	if !strings.Contains(got[15169], "Google") {
		t.Errorf("asnNames() for AS15169 == %q, expected Google", got[15169])
	}
	if _, ok := got[64496]; ok {
		t.Errorf("asnNames() has a name for the documentation AS64496")
	}
}
