// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

type healthcheckOptions struct {
	Host string
	Port uint16
}

// HealthcheckOptions stores the command-line option values for the healthcheck
// command.
var HealthcheckOptions healthcheckOptions

func init() {
	RootCmd.AddCommand(healthcheckCmd)
	healthcheckCmd.Flags().Uint16VarP(&HealthcheckOptions.Port, "port", "p", 8080,
		"HTTP port for health check")
	healthcheckCmd.Flags().StringVarP(&HealthcheckOptions.Host, "host", "h", "localhost",
		"HTTP host for health check")
}

var healthcheckCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "Check healthness",
	Long:  `Check if Akvorado is alive using the builtin HTTP endpoint.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		resp, err := http.Get(fmt.Sprintf("http://%s:%d/api/v0/healthcheck",
			HealthcheckOptions.Host,
			HealthcheckOptions.Port))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		cmd.Println("ok")
		return nil
	},
}
