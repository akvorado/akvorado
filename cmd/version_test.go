// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd_test

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"akvorado/cmd"
	"akvorado/common/helpers"
)

func TestVersion(t *testing.T) {
	root := cmd.RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"version"})
	err := root.Execute()
	if err != nil {
		t.Errorf("`version` error:\n%+v", err)
	}
	want := []string{
		"akvorado dev",
		fmt.Sprintf("  Built with: %s", runtime.Version()),
		"",
	}
	got := strings.Split(buf.String(), "\n")[:len(want)]
	if diff := helpers.Diff(got, want); diff != "" {
		t.Errorf("`version` (-got, +want):\n%s", diff)
	}
}
