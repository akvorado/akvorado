// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
)

func TestHealthcheck(t *testing.T) {
	// Setup a fake service
	r := reporter.NewMock(t)
	config := httpserver.DefaultConfiguration()
	config.Listen = "127.0.0.1:0"
	h, err := httpserver.New(r, "mock-healthcheck-test", config, httpserver.Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, h)
	h.GinRouter.GET("/api/v0/healthcheck", r.HealthcheckHTTPHandler)

	for _, tc := range []struct {
		description string
		args        string
		ok          bool
	}{
		// We can't really know if it works with no args, because other tests may be running in parallel.
		{
			description: "HTTP test",
			args:        fmt.Sprintf("--http %s", h.LocalAddr().String()),
			ok:          true,
		}, {
			description: "failing HTTP test",
			args:        "--http 127.0.0.1:0",
			ok:          false,
		}, {
			description: "unix test",
			args:        "--service mock-healthcheck-test",
			ok:          true,
		}, {
			description: "failing unix test",
			args:        "--service not-mock-healthcheck-test",
			ok:          false,
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			if strings.HasPrefix(tc.args, "--service") && runtime.GOOS != "linux" {
				t.Skip("unsupported OS")
			}
			args := []string{"healthcheck"}
			args = append(args, strings.Split(tc.args, " ")...)
			root := RootCmd
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetArgs(args)
			HealthcheckOptions.HTTP = ""
			HealthcheckOptions.UnixService = ""
			t.Logf("args: %s", args)
			err := root.Execute()
			if err != nil && tc.ok {
				t.Errorf("`healthcheck` error:\n%+v", err)
			} else if err == nil && !tc.ok {
				t.Error("`healthcheck` did not error")
			}
		})
	}
}
