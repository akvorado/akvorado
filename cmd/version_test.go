// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd_test

import (
	"bytes"
	"fmt"
	"runtime"
	"slices"
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
		fmt.Sprintf("  Build setting GOARCH=%s", runtime.GOARCH),
		fmt.Sprintf("  Build setting GOOS=%s", runtime.GOOS),
		"",
	}
	got := strings.Split(buf.String(), "\n")
	got = slices.DeleteFunc(got, func(s string) bool {
		return strings.HasPrefix(s, "  Build setting") &&
			!strings.HasPrefix(s, "  Build setting GOOS") &&
			!strings.HasPrefix(s, "  Build setting GOARCH")
	})

	if diff := helpers.Diff(got[:len(want)], want); diff != "" {
		t.Errorf("`version` (-got, +want):\n%s", diff)
	}
}
