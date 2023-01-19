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
	"akvorado/common/schema"
	"akvorado/console"
	"akvorado/console/authentication"
	"akvorado/console/database"
)

// ConsoleConfiguration represents the configuration file for the console command.
type ConsoleConfiguration struct {
	Reporting  reporter.Configuration
	HTTP       http.Configuration
	Console    console.Configuration `mapstructure:",squash" yaml:",inline"`
	ClickHouse clickhousedb.Configuration
	Auth       authentication.Configuration
	Database   database.Configuration
	Schema     schema.Configuration
}

// Reset resets the console configuration to its default value.
func (c *ConsoleConfiguration) Reset() {
	*c = ConsoleConfiguration{
		HTTP:       http.DefaultConfiguration(),
		Reporting:  reporter.DefaultConfiguration(),
		Console:    console.DefaultConfiguration(),
		ClickHouse: clickhousedb.DefaultConfiguration(),
		Auth:       authentication.DefaultConfiguration(),
		Database:   database.DefaultConfiguration(),
		Schema:     schema.DefaultConfiguration(),
	}
}

type consoleOptions struct {
	ConfigRelatedOptions
	CheckMode bool
}

// ConsoleOptions stores the command-line option values for the console
// command.
var ConsoleOptions consoleOptions

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Start Akvorado's console service",
	Long: `Akvorado is a Netflow/IPFIX collector. The console service exposes a web interface to
manage collected flows.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config := ConsoleConfiguration{}
		ConsoleOptions.Path = args[0]
		if err := ConsoleOptions.Parse(cmd.OutOrStdout(), "console", &config); err != nil {
			return err
		}
		config.Console.Version = Version

		r, err := reporter.New(config.Reporting)
		if err != nil {
			return fmt.Errorf("unable to initialize reporter: %w", err)
		}
		return consoleStart(r, config, ConsoleOptions.CheckMode)
	},
}

func init() {
	RootCmd.AddCommand(consoleCmd)
	consoleCmd.Flags().BoolVarP(&ConsoleOptions.ConfigRelatedOptions.Dump, "dump", "D", false,
		"Dump configuration before starting")
	consoleCmd.Flags().BoolVarP(&ConsoleOptions.CheckMode, "check", "C", false,
		"Check configuration, but does not start")
}

func consoleStart(r *reporter.Reporter, config ConsoleConfiguration, checkOnly bool) error {
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
	clickhouseComponent, err := clickhousedb.New(r, config.ClickHouse, clickhousedb.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize ClickHouse component: %w", err)
	}
	authenticationComponent, err := authentication.New(r, config.Auth)
	if err != nil {
		return fmt.Errorf("unable to initialize authentication component: %w", err)
	}
	databaseComponent, err := database.New(r, config.Database)
	if err != nil {
		return fmt.Errorf("unable to initialize database component: %w", err)
	}
	schemaComponent, err := schema.New(config.Schema)
	if err != nil {
		return fmt.Errorf("unable to initialize schema component: %w", err)
	}
	consoleComponent, err := console.New(r, config.Console, console.Dependencies{
		Daemon:       daemonComponent,
		HTTP:         httpComponent,
		ClickHouseDB: clickhouseComponent,
		Auth:         authenticationComponent,
		Database:     databaseComponent,
		Schema:       schemaComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize console component: %w", err)
	}

	// Expose some informations and metrics
	addCommonHTTPHandlers(r, "console", httpComponent)
	versionMetrics(r)

	// If we only asked for a check, stop here.
	if checkOnly {
		return nil
	}

	// Start all the components.
	components := []interface{}{
		httpComponent,
		clickhouseComponent,
		authenticationComponent,
		databaseComponent,
		consoleComponent,
	}
	return StartStopComponents(r, daemonComponent, components)
}
