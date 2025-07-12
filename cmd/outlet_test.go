// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"testing"

	"akvorado/common/reporter"
)

func TestOutletStart(t *testing.T) {
	r := reporter.NewMock(t)
	config := OutletConfiguration{}
	config.Reset()
	if err := outletStart(r, config, true); err != nil {
		t.Fatalf("outletStart() error:\n%+v", err)
	}
}

func TestOutlet(t *testing.T) {
	root := RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"outlet", "--check", "/dev/null"})
	err := root.Execute()
	if err != nil {
		t.Errorf("`outlet` error:\n%+v", err)
	}
}
