// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"strings"
	"testing"

	"akvorado/common/helpers"
)

func TestDispatchArgs(t *testing.T) {
	cases := []struct {
		Name string
		In   []string
		Want []string
	}{
		{
			Name: "plain akvorado",
			In:   []string{"/usr/bin/akvorado"},
			Want: []string{"/usr/bin/akvorado"},
		}, {
			Name: "plain akvorado with subcommand",
			In:   []string{"akvorado", "inlet", "/dev/null"},
			Want: []string{"akvorado", "inlet", "/dev/null"},
		}, {
			Name: "akvorado-inlet with config",
			In:   []string{"akvorado-inlet", "/dev/null"},
			Want: []string{"akvorado-inlet", "inlet", "/dev/null"},
		}, {
			Name: "akvorado-inlet with flags",
			In:   []string{"/usr/bin/akvorado-inlet", "--check", "/dev/null"},
			Want: []string{"/usr/bin/akvorado-inlet", "inlet", "--check", "/dev/null"},
		}, {
			Name: "akvorado-inlet version",
			In:   []string{"akvorado-inlet", "version"},
			Want: []string{"akvorado-inlet", "version"},
		}, {
			Name: "akvorado-inlet healthcheck",
			In:   []string{"akvorado-inlet", "healthcheck"},
			Want: []string{"akvorado-inlet", "healthcheck"},
		}, {
			Name: "unrelated binary name",
			In:   []string{"something-else", "something"},
			Want: []string{"something-else", "something"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := dispatchArgs(tc.In)
			if diff := helpers.Diff(got, tc.Want); diff != "" {
				t.Errorf("dispatchArgs(%v) (-got, +want):\n%s", tc.In, diff)
			}
		})
	}
}

func TestAkvoradoInletInvocation(t *testing.T) {
	args := dispatchArgs([]string{"akvorado-inlet", "--check", "/dev/null"})
	root := RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs(args[1:])
	if err := root.Execute(); err != nil {
		t.Errorf("`akvorado-inlet` error:\n%+v", err)
	}
}

func TestAkvoradoInletVersion(t *testing.T) {
	args := dispatchArgs([]string{"akvorado-inlet", "version"})
	root := RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs(args[1:])
	if err := root.Execute(); err != nil {
		t.Errorf("`akvorado-inlet version` error:\n%+v", err)
	}
	if !strings.Contains(buf.String(), "akvorado ") {
		t.Errorf("`akvorado-inlet version` output missing version line: %q", buf.String())
	}
}
