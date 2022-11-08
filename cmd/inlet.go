// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"
	"akvorado/inlet/bmp"
	"akvorado/inlet/core"
	"akvorado/inlet/flow"
	"akvorado/inlet/geoip"
	"akvorado/inlet/kafka"
	"akvorado/inlet/snmp"
)

// InletConfiguration represents the configuration file for the inlet command.
type InletConfiguration struct {
	Reporting reporter.Configuration
	HTTP      http.Configuration
	Flow      flow.Configuration
	SNMP      snmp.Configuration
	BMP       bmp.Configuration
	GeoIP     geoip.Configuration
	Kafka     kafka.Configuration
	Core      core.Configuration
}

// Reset resets the configuration for the inlet command to its default value.
func (c *InletConfiguration) Reset() {
	*c = InletConfiguration{
		HTTP:      http.DefaultConfiguration(),
		Reporting: reporter.DefaultConfiguration(),
		Flow:      flow.DefaultConfiguration(),
		SNMP:      snmp.DefaultConfiguration(),
		BMP:       bmp.DefaultConfiguration(),
		GeoIP:     geoip.DefaultConfiguration(),
		Kafka:     kafka.DefaultConfiguration(),
		Core:      core.DefaultConfiguration(),
	}
}

type inletOptions struct {
	ConfigRelatedOptions
	CheckMode bool
}

// InletOptions stores the command-line option values for the inlet
// command.
var InletOptions inletOptions

var inletCmd = &cobra.Command{
	Use:   "inlet",
	Short: "Start Akvorado's inlet service",
	Long: `Akvorado is a Netflow/IPFIX collector. The inlet service handles flow ingestion,
enrichment and export to Kafka.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config := InletConfiguration{}
		InletOptions.Path = args[0]
		if err := InletOptions.Parse(cmd.OutOrStdout(), "inlet", &config); err != nil {
			return err
		}

		r, err := reporter.New(config.Reporting)
		if err != nil {
			return fmt.Errorf("unable to initialize reporter: %w", err)
		}
		return inletStart(r, config, InletOptions.CheckMode)
	},
}

func init() {
	RootCmd.AddCommand(inletCmd)
	inletCmd.Flags().BoolVarP(&InletOptions.ConfigRelatedOptions.Dump, "dump", "D", false,
		"Dump configuration before starting")
	inletCmd.Flags().BoolVarP(&InletOptions.CheckMode, "check", "C", false,
		"Check configuration, but does not start")
}

func inletStart(r *reporter.Reporter, config InletConfiguration, checkOnly bool) error {
	// Initialize the various components
	daemonComponent, err := daemon.New(r)
	if err != nil {
		return fmt.Errorf("unable to initialize daemon component: %w", err)
	}
	httpComponent, err := http.New(r, config.HTTP, http.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize http component: %w", err)
	}
	flowComponent, err := flow.New(r, config.Flow, flow.Dependencies{
		Daemon: daemonComponent,
		HTTP:   httpComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize flow component: %w", err)
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
	geoipComponent, err := geoip.New(r, config.GeoIP, geoip.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize GeoIP component: %w", err)
	}
	kafkaComponent, err := kafka.New(r, config.Kafka, kafka.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize Kafka component: %w", err)
	}
	coreComponent, err := core.New(r, config.Core, core.Dependencies{
		Daemon: daemonComponent,
		Flow:   flowComponent,
		SNMP:   snmpComponent,
		BMP:    bmpComponent,
		GeoIP:  geoipComponent,
		Kafka:  kafkaComponent,
		HTTP:   httpComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize core component: %w", err)
	}

	// Expose some informations and metrics
	addCommonHTTPHandlers(r, "inlet", httpComponent)
	versionMetrics(r)

	// If we only asked for a check, stop here.
	if checkOnly {
		return nil
	}

	// Start all the components.
	components := []interface{}{
		httpComponent,
		snmpComponent,
		bmpComponent,
		geoipComponent,
		kafkaComponent,
		coreComponent,
		flowComponent,
	}
	return StartStopComponents(r, daemonComponent, components)
}
