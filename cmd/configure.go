package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"
	"akvorado/configure/clickhouse"
	"akvorado/configure/kafka"
)

// ConfigureConfiguration represents the configuration file for the configure command.
type ConfigureConfiguration struct {
	Reporting  reporter.Configuration
	HTTP       http.Configuration
	Clickhouse clickhouse.Configuration
	Kafka      kafka.Configuration
}

// DefaultConfigureConfiguration is the default configuration for the configure command.
var DefaultConfigureConfiguration = ConfigureConfiguration{
	HTTP:       http.DefaultConfiguration,
	Reporting:  reporter.DefaultConfiguration,
	Clickhouse: clickhouse.DefaultConfiguration,
	Kafka:      kafka.DefaultConfiguration,
}

type configureOptions struct {
	ConfigRelatedOptions
	CheckMode bool
}

// ConfigureOptions stores the command-line option values for the configure
// command.
var ConfigureOptions configureOptions

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Start Akvorado's configure service",
	Long: `Akvorado is a Netflow/IPFIX collector. The configure service configure external
components: Kafka and Clickhouse.`,
	Args: cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		config := DefaultConfigureConfiguration
		ConfigureOptions.BeforeDump = func() {
			if config.Clickhouse.Kafka.Topic == "" {
				fmt.Println(config.Kafka.Configuration)
				config.Clickhouse.Kafka.Configuration = config.Kafka.Configuration
			}
		}
		if err := ConfigureOptions.Parse(cmd.OutOrStdout(), "configure", &config); err != nil {
			return err
		}

		r, err := reporter.New(config.Reporting)
		if err != nil {
			return fmt.Errorf("unable to initialize reporter: %w", err)
		}
		return configureStart(r, config, ConfigureOptions.CheckMode)
	},
}

func init() {
	RootCmd.AddCommand(configureCmd)
	configureCmd.Flags().StringVarP(&ConfigureOptions.ConfigRelatedOptions.Path, "config", "c", "",
		"Configuration file")
	configureCmd.Flags().BoolVarP(&ConfigureOptions.ConfigRelatedOptions.Dump, "dump", "D", false,
		"Dump configuration before starting")
	configureCmd.Flags().BoolVarP(&ConfigureOptions.CheckMode, "check", "C", false,
		"Check configuration, but does not start")
}

func configureStart(r *reporter.Reporter, config ConfigureConfiguration, checkOnly bool) error {
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
	clickhouseComponent, err := clickhouse.New(r, config.Clickhouse, clickhouse.Dependencies{
		Daemon: daemonComponent,
		HTTP:   httpComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize clickhouse component: %w", err)
	}

	// Expose some informations and metrics
	addCommonHTTPHandlers(r, "configure", httpComponent)
	versionMetrics(r)

	// If we only asked for a check, stop here.
	if checkOnly {
		return nil
	}

	// Start all the components.
	components := []interface{}{
		httpComponent,
		clickhouseComponent,
		kafkaComponent,
	}
	return StartStopComponents(r, daemonComponent, components)
}
