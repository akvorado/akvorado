package cmd

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"

	"akvorado/core"
	"akvorado/daemon"
	"akvorado/flow"
	"akvorado/geoip"
	"akvorado/http"
	"akvorado/kafka"
	"akvorado/reporter"
	"akvorado/snmp"
)

type daemonConfiguration struct {
	Reporting reporter.Configuration
	HTTP      http.Configuration
	Flow      flow.Configuration
	SNMP      snmp.Configuration
	GeoIP     geoip.Configuration
	Kafka     kafka.Configuration
	Core      core.Configuration
}

var defaultDaemonConfiguration = daemonConfiguration{
	Reporting: reporter.DefaultConfiguration,
	HTTP:      http.DefaultConfiguration,
	Flow:      flow.DefaultConfiguration,
	SNMP:      snmp.DefaultConfiguration,
	GeoIP:     geoip.DefaultConfiguration,
	Kafka:     kafka.DefaultConfiguration,
	Core:      core.DefaultConfiguration,
}
var daemonOptions struct {
	configurationFile string
	checkMode         bool
	dumpConfiguration bool
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start akvorado",
	Long: `Akvorado is a Netflow collector. It enriches flows with information from SNMP and GeoIP
and exports them to Kafka.`,
	Args: cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfgFile := daemonOptions.configurationFile; cfgFile != "" {
			viper.SetConfigFile(cfgFile)
			if err := viper.ReadInConfig(); err != nil {
				return fmt.Errorf("unable to read configuration file: %w", err)
			}
		}
		config := defaultDaemonConfiguration

		// Parse provided configuration
		if err := viper.Unmarshal(&config, func(c *mapstructure.DecoderConfig) {
			c.ErrorUnused = true
		}); err != nil {
			return fmt.Errorf("unable to parse configuration: %w", err)
		}

		// Dump configuration if requested
		if daemonOptions.dumpConfiguration {
			output, err := yaml.Marshal(config)
			if err != nil {
				return fmt.Errorf("unable to dump configuration: %w", err)
			}
			fmt.Printf("---\n%s\n", string(output))
		}

		r, err := reporter.New(config.Reporting)
		if err != nil {
			return fmt.Errorf("unable to initialize reporter: %w", err)
		}
		return daemonStart(r, config, daemonOptions.checkMode)
	},
}

func init() {
	RootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVarP(&daemonOptions.configurationFile, "config", "c", "",
		"Configuration file")
	serveCmd.Flags().BoolVarP(&daemonOptions.checkMode, "check", "C", false,
		"Check configuration, but does not start")
	serveCmd.Flags().BoolVarP(&daemonOptions.dumpConfiguration, "dump", "D", false,
		"Dump configuration before starting")
}

// daemonStart will start all components and manage daemon lifetime.
func daemonStart(r *reporter.Reporter, config daemonConfiguration, checkOnly bool) error {
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
		Snmp:   snmpComponent,
		GeoIP:  geoipComponent,
		Kafka:  kafkaComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize core component: %w", err)
	}

	// If we only asked for a check, stop here.
	if checkOnly {
		return nil
	}

	// Start all the components.
	httpComponent.AddHandler("/metrics", r.MetricsHTTPHandler())
	components := []interface{}{
		r,
		daemonComponent,
		httpComponent,
		flowComponent,
		snmpComponent,
		geoipComponent,
		kafkaComponent,
		coreComponent,
	}
	startedComponents := []interface{}{}
	defer func() {
		for _, cmp := range startedComponents {
			if stopperC, ok := cmp.(stopper); ok {
				if err := stopperC.Stop(); err != nil {
					r.Err(err).Msg("unable to stop component, ignoring")
				}
			}
		}
	}()
	for _, cmp := range components {
		if starterC, ok := cmp.(starter); ok {
			if err := starterC.Start(); err != nil {
				return fmt.Errorf("unable to start component: %w", err)
			}
		}
		startedComponents = append([]interface{}{cmp}, startedComponents...)
	}

	r.Info().
		Str("version", Version).Str("build-date", BuildDate).
		Msg("akvorado has started")

	select {
	case <-daemonComponent.Terminated():
		r.Info().Msg("stopping all components")
	}

	return nil
}

type starter interface {
	Start() error
}
type stopper interface {
	Stop() error
}
