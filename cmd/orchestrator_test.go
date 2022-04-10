package cmd

import (
	"testing"

	"akvorado/common/reporter"
)

func TestOrchestratorStart(t *testing.T) {
	r := reporter.NewMock(t)
	if err := orchestratorStart(r, DefaultOrchestratorConfiguration(), true); err != nil {
		t.Fatalf("orchestratorStart() error:\n%+v", err)
	}
}
