// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"
	"akvorado/orchestrator"
	"akvorado/orchestrator/clickhouse"
	"akvorado/orchestrator/kafka"
)

// OrchestratorConfiguration represents the configuration file for the orchestrator command.
type OrchestratorConfiguration struct {
	Reporting    reporter.Configuration
	HTTP         http.Configuration
	ClickHouseDB clickhousedb.Configuration `yaml:"-"`
	ClickHouse   clickhouse.Configuration
	Kafka        kafka.Configuration
	Orchestrator orchestrator.Configuration `mapstructure:",squash" yaml:",inline"`
	// Other service configurations
	Inlet   InletConfiguration
	Console ConsoleConfiguration
}

// DefaultOrchestratorConfiguration is the default configuration for the orchestrator command.
func DefaultOrchestratorConfiguration() OrchestratorConfiguration {
	return OrchestratorConfiguration{
		Reporting:    reporter.DefaultConfiguration(),
		HTTP:         http.DefaultConfiguration(),
		ClickHouseDB: clickhousedb.DefaultConfiguration(),
		ClickHouse:   clickhouse.DefaultConfiguration(),
		Kafka:        kafka.DefaultConfiguration(),
		Orchestrator: orchestrator.DefaultConfiguration(),
		// Other service configurations
		Inlet:   DefaultInletConfiguration(),
		Console: DefaultConsoleConfiguration(),
	}
}

type orchestratorOptions struct {
	ConfigRelatedOptions
	CheckMode bool
}

// OrchestratorOptions stores the command-line option values for the orchestrator
// command.
var OrchestratorOptions orchestratorOptions

var orchestratorCmd = &cobra.Command{
	Use:   "orchestrator",
	Short: "Start Akvorado's orchestrator service",
	Long: `Akvorado is a Netflow/IPFIX collector. The orchestrator service configures external
components and centralizes configuration of the various other components.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config := DefaultOrchestratorConfiguration()
		OrchestratorOptions.Path = args[0]
		OrchestratorOptions.BeforeDump = func() {
			// Override some parts of the configuration
			config.ClickHouseDB = config.ClickHouse.Configuration
			config.ClickHouse.Kafka.Configuration = config.Kafka.Configuration
			config.Inlet.Kafka.Configuration = config.Kafka.Configuration
			config.Console.ClickHouse = config.ClickHouse.Configuration
		}
		if err := OrchestratorOptions.Parse(cmd.OutOrStdout(), "orchestrator", &config); err != nil {
			return err
		}

		r, err := reporter.New(config.Reporting)
		if err != nil {
			return fmt.Errorf("unable to initialize reporter: %w", err)
		}
		return orchestratorStart(r, config, OrchestratorOptions.CheckMode)
	},
}

func init() {
	RootCmd.AddCommand(orchestratorCmd)
	orchestratorCmd.Flags().BoolVarP(&OrchestratorOptions.ConfigRelatedOptions.Dump, "dump", "D", false,
		"Dump configuration before starting")
	orchestratorCmd.Flags().BoolVarP(&OrchestratorOptions.CheckMode, "check", "C", false,
		"Check configuration, but does not start")
}

func orchestratorStart(r *reporter.Reporter, config OrchestratorConfiguration, checkOnly bool) error {
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
	kafkaComponent, err := kafka.New(r, config.Kafka)
	if err != nil {
		return fmt.Errorf("unable to initialize kafka component: %w", err)
	}
	clickhouseDBComponent, err := clickhousedb.New(r, config.ClickHouseDB, clickhousedb.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize ClickHouse component: %w", err)
	}
	clickhouseComponent, err := clickhouse.New(r, config.ClickHouse, clickhouse.Dependencies{
		Daemon:     daemonComponent,
		HTTP:       httpComponent,
		ClickHouse: clickhouseDBComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize clickhouse component: %w", err)
	}
	orchestratorComponent, err := orchestrator.New(r, config.Orchestrator, orchestrator.Dependencies{
		HTTP: httpComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize orchestrator component: %w", err)
	}
	orchestratorComponent.RegisterConfiguration(orchestrator.InletService, config.Inlet)
	orchestratorComponent.RegisterConfiguration(orchestrator.ConsoleService, config.Console)

	// Expose some informations and metrics
	addCommonHTTPHandlers(r, "orchestrator", httpComponent)
	versionMetrics(r)

	// If we only asked for a check, stop here.
	if checkOnly {
		return nil
	}

	// Start all the components.
	components := []interface{}{
		httpComponent,
		clickhouseDBComponent,
		clickhouseComponent,
		kafkaComponent,
	}
	return StartStopComponents(r, daemonComponent, components)
}
