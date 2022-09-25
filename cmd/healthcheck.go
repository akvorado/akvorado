// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"net/http"

	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(healthcheckCmd)
}

var healthcheckCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "Check healthness",
	Long:  `Check if Akvorado is alive using the builtin HTTP endpoint.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := http.Get("http://localhost:8080/api/v0/healthcheck")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		cmd.Println("ok")
		return nil
	},
}
