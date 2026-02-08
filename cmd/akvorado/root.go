// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package main handles the command-line interface for akvorado
package main

import (
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"akvorado/cmd"
	"akvorado/common/helpers"
	"akvorado/common/reporter"

	"github.com/spf13/cobra"
)

var (
	debug      bool
	missedLogs atomic.Uint64
	startTime  time.Time
)

// RootCmd is the root for all commands
var RootCmd = &cobra.Command{
	Use:   "akvorado",
	Short: "Flow collector, enricher and visualizer",
	PersistentPreRun: func(*cobra.Command, []string) {
		cmd.SetupLogging(debug)
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
		Help: "Akvorado build information.",
	}, []string{"version", "compiler"}).
		WithLabelValues(helpers.AkvoradoVersion, runtime.Version()).Set(1)
	r.GaugeFunc(reporter.GaugeOpts{
		Name: "uptime_seconds",
		Help: "Number of seconds the application is running.",
	}, func() float64 {
		return time.Since(startTime).Seconds()
	})
}

func init() {
	startTime = time.Now()
	RootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false,
		"Enable debug logs")
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		os.Exit(1)
	}
}
