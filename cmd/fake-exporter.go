// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"
	"akvorado/fakeexporter"
	"akvorado/fakeexporter/flows"
	"akvorado/fakeexporter/snmp"
)

// FakeExporterConfiguration represents the configuration file for the fake exporter command.
type FakeExporterConfiguration struct {
	Reporting    reporter.Configuration
	HTTP         http.Configuration
	FakeExporter fakeexporter.Configuration `mapstructure:",squash" yaml:",inline"`
	SNMP         snmp.Configuration
	Flows        flows.Configuration
}

// Reset sets the default configuration for the fake exporter command.
func (c *FakeExporterConfiguration) Reset() {
	*c = FakeExporterConfiguration{
		HTTP:         http.DefaultConfiguration(),
		Reporting:    reporter.DefaultConfiguration(),
		FakeExporter: fakeexporter.DefaultConfiguration(),
	}
}

type fakeExporterOptions struct {
	ConfigRelatedOptions
	CheckMode bool
}

// FakeExporterOptions stores the command-line option values for the
// fake exporter command.
var FakeExporterOptions fakeExporterOptions

var fakeExporterCmd = &cobra.Command{
	Use:   "fake-exporter",
	Short: "Start a synthetic exporter",
	Long: `For demo and testing purpose, this service exports synthetic flows
and answers SNMP requests.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config := FakeExporterConfiguration{}
		FakeExporterOptions.Path = args[0]
		if err := FakeExporterOptions.Parse(cmd.OutOrStdout(), "fake-exporter", &config); err != nil {
			return err
		}

		r, err := reporter.New(config.Reporting)
		if err != nil {
			return fmt.Errorf("unable to initialize reporter: %w", err)
		}
		return fakeExporterStart(r, config, FakeExporterOptions.CheckMode)
	},
}

func init() {
	RootCmd.AddCommand(fakeExporterCmd)
	fakeExporterCmd.Flags().BoolVarP(&FakeExporterOptions.ConfigRelatedOptions.Dump, "dump", "D", false,
		"Dump configuration before starting")
	fakeExporterCmd.Flags().BoolVarP(&FakeExporterOptions.CheckMode, "check", "C", false,
		"Check configuration, but does not start")
}

func fakeExporterStart(r *reporter.Reporter, config FakeExporterConfiguration, checkOnly bool) error {
	daemonComponent, err := daemon.New(r)
	if err != nil {
		return fmt.Errorf("unable to initialize daemon component: %w", err)
	}
	httpComponent, err := http.New(r, config.HTTP, http.Dependencies{
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
	flowsComponent, err := flows.New(r, config.Flows, flows.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize flows component: %w", err)
	}
	fakeExporterComponent, err := fakeexporter.New(r, config.FakeExporter, fakeexporter.Dependencies{
		SNMP:  snmpComponent,
		Flows: flowsComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize exporter component: %w", err)
	}

	// Expose some informations and metrics
	addCommonHTTPHandlers(r, "fake-exporter", httpComponent)
	versionMetrics(r)

	// If we only asked for a check, stop here.
	if checkOnly {
		return nil
	}

	// Start all the components.
	components := []interface{}{
		httpComponent,
		snmpComponent,
		flowsComponent,
		fakeExporterComponent,
	}
	return StartStopComponents(r, daemonComponent, components)
}
