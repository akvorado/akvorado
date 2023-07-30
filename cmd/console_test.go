// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"testing"

	"akvorado/common/reporter"
)

func TestConsoleStart(t *testing.T) {
	r := reporter.NewMock(t)
	config := ConsoleConfiguration{}
	config.Reset()
	if err := consoleStart(r, config, true); err != nil {
		t.Fatalf("consoleStart() error:\n%+v", err)
	}
}

func TestConsole(t *testing.T) {
	root := RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"console", "--check", "/dev/null"})
	err := root.Execute()
	if err != nil {
		t.Errorf("`console` error:\n%+v", err)
	}
}
