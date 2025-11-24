// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime"

	"github.com/spf13/cobra"
)

type healthcheckOptions struct {
	HTTP        string
	UnixService string
}

// HealthcheckOptions stores the command-line option values for the healthcheck
// command.
var HealthcheckOptions healthcheckOptions

func init() {
	RootCmd.AddCommand(healthcheckCmd)
	if runtime.GOOS == "linux" {
		// On Linux, use Unix sockets
		healthcheckCmd.Flags().StringVarP(&HealthcheckOptions.HTTP, "http", "", "",
			"HTTP host:port for health check")
		healthcheckCmd.Flags().StringVarP(&HealthcheckOptions.UnixService, "service", "", "",
			"Service to query over Unix socket")
	} else {
		// On other OS, use HTTP
		healthcheckCmd.Flags().StringVarP(&HealthcheckOptions.HTTP, "http", "", "localhost:8080",
			"HTTP host:port for health check")
	}
}

var healthcheckCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "Check healthness",
	Long: `Check if Akvorado is alive using the builtin HTTP endpoint.
The service can be checked over Unix socket (by default), or over HTTP`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		httpc := http.Client{}
		if HealthcheckOptions.HTTP == "" {
			unixSocket := "@akvorado"
			if HealthcheckOptions.UnixService != "" {
				unixSocket = fmt.Sprintf("%s/%s", unixSocket, HealthcheckOptions.UnixService)
			}
			httpc.Transport = &http.Transport{
				DialContext: func(context.Context, string, string) (net.Conn, error) {
					return net.Dial("unix", unixSocket)
				},
			}
		}
		resp, err := httpc.Get(fmt.Sprintf("http://%s/api/v0/healthcheck",
			cmp.Or(HealthcheckOptions.HTTP, "unix")))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		cmd.Println("ok")
		return nil
	},
}
