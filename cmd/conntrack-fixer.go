// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build linux

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"akvorado/common/daemon"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/conntrackfixer"
)

var conntrackFixerCmd = &cobra.Command{
	Use:   "conntrack-fixer",
	Short: "Clean conntrack for UDP ports",
	Long: `This helper cleans the conntrack entries for the UDP ports exposed by
containers started with the label "akvorado.conntrack.fix=1".`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// This is a simplified service which is not configurable.
		r, err := reporter.New(reporter.DefaultConfiguration())
		if err != nil {
			return fmt.Errorf("unable to initialize reporter: %w", err)
		}
		daemonComponent, err := daemon.New(r)
		if err != nil {
			return fmt.Errorf("unable to initialize daemon component: %w", err)
		}
		httpConfiguration := httpserver.DefaultConfiguration()
		httpConfiguration.Listen = "127.0.0.1:0" // Run inside host network namespace, can't use 8080
		httpComponent, err := httpserver.New(r, httpConfiguration, httpserver.Dependencies{
			Daemon: daemonComponent,
		})
		if err != nil {
			return fmt.Errorf("unable to initialize HTTP component: %w", err)
		}
		conntrackFixerComponent, err := conntrackfixer.New(r,
			conntrackfixer.Dependencies{
				Daemon: daemonComponent,
				HTTP:   httpComponent,
			})
		if err != nil {
			return fmt.Errorf("unable to initialize conntrack fixer component: %w", err)
		}
		addCommonHTTPHandlers(r, "conntrack-fixer", httpComponent)
		versionMetrics(r)

		components := []interface{}{
			httpComponent,
			conntrackFixerComponent,
		}
		return StartStopComponents(r, daemonComponent, components)
	},
}

func init() {
	RootCmd.AddCommand(conntrackFixerCmd)
}
