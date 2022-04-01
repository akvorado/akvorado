package cmd

import (
	"testing"

	"akvorado/common/reporter"
)

func TestConfigureStart(t *testing.T) {
	r := reporter.NewMock(t)
	if err := configureStart(r, DefaultConfigureConfiguration, true); err != nil {
		t.Fatalf("configureStart() error:\n%+v", err)
	}
}
