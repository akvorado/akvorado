// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"testing"

	"akvorado/common/reporter"
)

func TestInletStart(t *testing.T) {
	r := reporter.NewMock(t)
	config := InletConfiguration{}
	config.Reset()
	if err := inletStart(r, config, true); err != nil {
		t.Fatalf("inletStart() error:\n%+v", err)
	}
}

func TestInlet(t *testing.T) {
	root := RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"inlet", "--check", "/dev/null"})
	err := root.Execute()
	if err != nil {
		t.Errorf("`inlet` error:\n%+v", err)
	}
}
