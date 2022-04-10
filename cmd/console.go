package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"
	"akvorado/console"
)

// ConsoleConfiguration represents the configuration file for the console command.
type ConsoleConfiguration struct {
	Reporting reporter.Configuration
	HTTP      http.Configuration
	Console   console.Configuration
}

// DefaultConsoleConfiguration is the default configuration for the console command.
func DefaultConsoleConfiguration() ConsoleConfiguration {
	return ConsoleConfiguration{
		HTTP:      http.DefaultConfiguration(),
		Reporting: reporter.DefaultConfiguration(),
		Console:   console.DefaultConfiguration(),
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
		config := DefaultConsoleConfiguration()
		ConsoleOptions.Path = args[0]
		if err := ConsoleOptions.Parse(cmd.OutOrStdout(), "console", &config); err != nil {
			return err
		}

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
	consoleComponent, err := console.New(r, config.Console, console.Dependencies{
		HTTP: httpComponent,
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
		consoleComponent,
	}
	return StartStopComponents(r, daemonComponent, components)
}
