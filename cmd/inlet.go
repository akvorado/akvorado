package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"
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
	GeoIP     geoip.Configuration
	Kafka     kafka.Configuration
	Core      core.Configuration
}

// DefaultInletConfiguration is the default configuration for the inlet command.
func DefaultInletConfiguration() InletConfiguration {
	return InletConfiguration{
		HTTP:      http.DefaultConfiguration(),
		Reporting: reporter.DefaultConfiguration(),
		Flow:      flow.DefaultConfiguration(),
		SNMP:      snmp.DefaultConfiguration(),
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
hydration and export to Kafka.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config := DefaultInletConfiguration()
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
		geoipComponent,
		kafkaComponent,
		coreComponent,
		flowComponent,
	}
	return StartStopComponents(r, daemonComponent, components)
}
