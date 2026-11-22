// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDemoExporterTrain(t *testing.T) {
	config := `---
start: 2026-08-01T00:00:00Z
end: 2026-08-08T00:00:00Z
anonymization:
  ipv4-prefix: 98.0.0.0/8
  ipv6-prefix: 3fff::/20
output: model.bin
`
	path := filepath.Join(t.TempDir(), "train.yaml")
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatalf("WriteFile() error:\n%+v", err)
	}
	root := RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"demo-exporter", "train", "--check", path})
	if err := root.Execute(); err != nil {
		t.Fatalf("`demo-exporter train` error:\n%+v", err)
	}
}

func TestDemoExporterTrainInvalid(t *testing.T) {
	root := RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"demo-exporter", "train", "--check", "/dev/null"})
	if err := root.Execute(); err == nil {
		t.Fatal("`demo-exporter train` should produce an error")
	}
}
