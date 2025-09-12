// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package cmd handles the command-line interface for akvorado
package cmd

import (
	"os"
	"runtime"
	"sync/atomic"

	"akvorado/common/helpers"
	"akvorado/common/reporter"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	debug      bool
	missedLogs atomic.Uint64
)

// RootCmd is the root for all commands
var RootCmd = &cobra.Command{
	Use:   "akvorado",
	Short: "Flow collector, enricher and visualizer",
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		if isatty.IsTerminal(os.Stdout.Fd()) {
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		} else {
			w := diode.NewWriter(os.Stdout, 1000, 0, func(missed int) {
				missedLogs.Add(uint64(missed))
			})
			log.Logger = zerolog.New(w).With().Timestamp().Logger()
		}
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		if debug {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		}
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func moreMetrics(r *reporter.Reporter) {
	versionMetrics(r)
	r.GaugeFunc(reporter.GaugeOpts{
		Name: "dropped_log_messages",
		Help: "Number of log messages dropped.",
	}, func() float64 {
		return float64(missedLogs.Load())
	})
	r.GaugeVec(reporter.GaugeOpts{
		Name: "info",
		Help: "Akvorado build information",
	}, []string{"version", "compiler"}).
		WithLabelValues(helpers.AkvoradoVersion, runtime.Version()).Set(1)
}

func init() {
	RootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false,
		"Enable debug logs")
}
