// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"testing"

	"akvorado/common/reporter"
)

func TestOrchestratorStart(t *testing.T) {
	r := reporter.NewMock(t)
	config := OrchestratorConfiguration{}
	config.Reset()
	if err := orchestratorStart(r, config, true); err != nil {
		t.Fatalf("orchestratorStart() error:\n%+v", err)
	}
}

func TestOrchestratorFullConfig(t *testing.T) {
	root := RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"orchestrator", "--dump", "--check", "../akvorado.yaml"})
	err := root.Execute()
	if err != nil {
		t.Errorf("`orchestrator` command error:\n%+v", err)
	}
}
