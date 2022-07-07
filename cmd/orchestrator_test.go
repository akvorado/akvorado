// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
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
