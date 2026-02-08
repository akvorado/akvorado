// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package main handles the command-line interface for the helper tool.
package main

import (
	"fmt"
	"os"

	"akvorado/cmd"

	"github.com/spf13/cobra"
)

var debug bool

// RootCmd is the root for all commands
var RootCmd = &cobra.Command{
	Use:   "helper",
	Short: "Helper tool for Akvorado",
	PersistentPreRun: func(*cobra.Command, []string) {
		cmd.SetupLogging(debug)
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	RootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false,
		"Enable debug logs")
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		os.Exit(1)
	}
}
