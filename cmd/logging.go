// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package cmd handles the command-line interface for helper tool.
package cmd

import (
	"os"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// SetupLogging is to be executed to setup logging for command-line tools.
func SetupLogging(debug bool) {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	} else {
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}
