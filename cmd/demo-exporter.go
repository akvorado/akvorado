// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"akvorado/common/daemon"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/demoexporter"
	"akvorado/demoexporter/bmp"
	"akvorado/demoexporter/flows"
	"akvorado/demoexporter/snmp"
)

// DemoExporterConfiguration represents the configuration file for the demo exporter command.
type DemoExporterConfiguration struct {
	Reporting    reporter.Configuration
	HTTP         httpserver.Configuration
	DemoExporter demoexporter.Configuration `mapstructure:",squash" yaml:",inline"`
	SNMP         snmp.Configuration
	BMP          bmp.Configuration
	Flows        flows.Configuration
}

// Reset sets the default configuration for the demo exporter command.
func (c *DemoExporterConfiguration) Reset() {
	*c = DemoExporterConfiguration{
		HTTP:         httpserver.DefaultConfiguration(),
		Reporting:    reporter.DefaultConfiguration(),
		DemoExporter: demoexporter.DefaultConfiguration(),
		SNMP:         snmp.DefaultConfiguration(),
		BMP:          bmp.DefaultConfiguration(),
		Flows:        flows.DefaultConfiguration(),
	}
}

type demoExporterOptions struct {
	ConfigRelatedOptions
	CheckMode bool
}

// DemoExporterOptions stores the command-line option values for the
// demo exporter command.
var DemoExporterOptions demoExporterOptions

var demoExporterCmd = &cobra.Command{
	Use:   "demo-exporter",
	Short: "Start a synthetic exporter",
	Long: `For demo and testing purpose, this service exports synthetic flows
and answers SNMP requests.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config := DemoExporterConfiguration{}
		DemoExporterOptions.Path = args[0]
		if err := DemoExporterOptions.Parse(cmd.OutOrStdout(), "demo-exporter", &config); err != nil {
			return err
		}

		r, err := reporter.New(config.Reporting)
		if err != nil {
			return fmt.Errorf("unable to initialize reporter: %w", err)
		}
		return demoExporterStart(r, config, DemoExporterOptions.CheckMode)
	},
}

func init() {
	RootCmd.AddCommand(demoExporterCmd)
	demoExporterCmd.Flags().BoolVarP(&DemoExporterOptions.ConfigRelatedOptions.Dump, "dump", "D", false,
		"Dump configuration before starting")
	demoExporterCmd.Flags().BoolVarP(&DemoExporterOptions.CheckMode, "check", "C", false,
		"Check configuration, but does not start")
}

func demoExporterStart(r *reporter.Reporter, config DemoExporterConfiguration, checkOnly bool) error {
	daemonComponent, err := daemon.New(r)
	if err != nil {
		return fmt.Errorf("unable to initialize daemon component: %w", err)
	}
	httpComponent, err := httpserver.New(r, config.HTTP, httpserver.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize HTTP component: %w", err)
	}
	snmpComponent, err := snmp.New(r, config.SNMP, snmp.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize SNMP component: %w", err)
	}
	bmpComponent, err := bmp.New(r, config.BMP, bmp.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize BMP component: %w", err)
	}
	flowsComponent, err := flows.New(r, config.Flows, flows.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize flows component: %w", err)
	}
	demoExporterComponent, err := demoexporter.New(r, config.DemoExporter, demoexporter.Dependencies{
		SNMP:  snmpComponent,
		Flows: flowsComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize exporter component: %w", err)
	}

	// Expose some information and metrics
	addCommonHTTPHandlers(r, "demo-exporter", httpComponent)
	versionMetrics(r)

	// If we only asked for a check, stop here.
	if checkOnly {
		return nil
	}

	// Start all the components.
	components := []any{
		httpComponent,
		snmpComponent,
		bmpComponent,
		flowsComponent,
		demoExporterComponent,
	}
	return StartStopComponents(r, daemonComponent, components)
}
