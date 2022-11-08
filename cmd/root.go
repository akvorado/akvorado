// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package cmd handles the command-line interface for akvorado
package cmd

import (
	"os"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var debug bool

// RootCmd is the root for all commands
var RootCmd = &cobra.Command{
	Use:   "akvorado",
	Short: "Flow collector, enricher and visualizer",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if isatty.IsTerminal(os.Stdout.Fd()) {
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		} else {
			log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
		}
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		if debug {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		}
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	RootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false,
		"Enable debug logs")
}
