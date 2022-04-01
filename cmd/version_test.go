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
		"  Build date: unknown",
		fmt.Sprintf("  Built with: %s", runtime.Version()),
		"",
	}
	got := strings.Split(buf.String(), "\n")
	if diff := helpers.Diff(got, want); diff != "" {
		t.Errorf("`version` (-got, +want):\n%s", diff)
	}
}
