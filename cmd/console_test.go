package cmd

import (
	"testing"

	"akvorado/common/reporter"
)

func TestConsoleStart(t *testing.T) {
	r := reporter.NewMock(t)
	if err := consoleStart(r, DefaultConsoleConfiguration(), true); err != nil {
		t.Fatalf("consoleStart() error:\n%+v", err)
	}
}
