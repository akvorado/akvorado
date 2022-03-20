package cmd

import (
	"encoding/json"
	"fmt"
	netHTTP "net/http"
	"runtime"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"

	"akvorado/clickhouse"
	"akvorado/core"
	"akvorado/daemon"
	"akvorado/flow"
	"akvorado/geoip"
	"akvorado/http"
	"akvorado/kafka"
	"akvorado/reporter"
	"akvorado/snmp"
	"akvorado/web"
)

// ServeConfiguration represents the configuration file for the serve command.
type ServeConfiguration struct {
	Reporting  reporter.Configuration
	HTTP       http.Configuration
	Flow       flow.Configuration
	SNMP       snmp.Configuration
	GeoIP      geoip.Configuration
	Kafka      kafka.Configuration
	Core       core.Configuration
	Web        web.Configuration
	Clickhouse clickhouse.Configuration
}

// DefaultServeConfiguration is the default configuration for the serve command.
var DefaultServeConfiguration = ServeConfiguration{
	Reporting:  reporter.DefaultConfiguration,
	HTTP:       http.DefaultConfiguration,
	Flow:       flow.DefaultConfiguration,
	SNMP:       snmp.DefaultConfiguration,
	GeoIP:      geoip.DefaultConfiguration,
	Kafka:      kafka.DefaultConfiguration,
	Core:       core.DefaultConfiguration,
	Web:        web.DefaultConfiguration,
	Clickhouse: clickhouse.DefaultConfiguration,
}

type serveOptions struct {
	configurationFile string
	checkMode         bool
	dumpConfiguration bool
}

// ServeOptions stores the command-line option values for the serve
// command.
var ServeOptions serveOptions

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start akvorado",
	Long: `Akvorado is a Netflow collector. It hydrates flows with information from SNMP and GeoIP
and exports them to Kafka.`,
	Args: cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfgFile := ServeOptions.configurationFile; cfgFile != "" {
			viper.SetConfigFile(cfgFile)
			if err := viper.ReadInConfig(); err != nil {
				return fmt.Errorf("unable to read configuration file: %w", err)
			}
		}
		config := DefaultServeConfiguration

		// Parse provided configuration
		if err := viper.Unmarshal(&config, func(c *mapstructure.DecoderConfig) {
			c.ErrorUnused = true
			c.MatchName = func(mapKey, fieldName string) bool {
				key := strings.ToLower(strings.ReplaceAll(mapKey, "-", ""))
				field := strings.ToLower(fieldName)
				return key == field
			}
			c.DecodeHook = mapstructure.ComposeDecodeHookFunc(
				mapstructure.TextUnmarshallerHookFunc(),
				mapstructure.StringToTimeDurationHookFunc(),
			)
		}); err != nil {
			return fmt.Errorf("unable to parse configuration: %w", err)
		}

		// Dump configuration if requested
		if ServeOptions.dumpConfiguration {
			output, err := yaml.Marshal(config)
			if err != nil {
				return fmt.Errorf("unable to dump configuration: %w", err)
			}
			cmd.Printf("---\n%s\n", string(output))
		}

		r, err := reporter.New(config.Reporting)
		if err != nil {
			return fmt.Errorf("unable to initialize reporter: %w", err)
		}
		return daemonStart(r, config, ServeOptions.checkMode)
	},
}

func init() {
	RootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVarP(&ServeOptions.configurationFile, "config", "c", "",
		"Configuration file")
	serveCmd.Flags().BoolVarP(&ServeOptions.checkMode, "check", "C", false,
		"Check configuration, but does not start")
	serveCmd.Flags().BoolVarP(&ServeOptions.dumpConfiguration, "dump", "D", false,
		"Dump configuration before starting")
}

// daemonStart will start all components and manage daemon lifetime.
func daemonStart(r *reporter.Reporter, config ServeConfiguration, checkOnly bool) error {
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
	clickhouseComponent, err := clickhouse.New(r, config.Clickhouse, clickhouse.Dependencies{
		HTTP: httpComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize Clickhouse component: %w", err)
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
	webComponent, err := web.New(r, config.Web, web.Dependencies{
		HTTP: httpComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize web component: %w", err)
	}

	// If we only asked for a check, stop here.
	if checkOnly {
		return nil
	}

	// Expose some informations and metrics
	httpComponent.AddHandler("/api/v0/metrics", r.MetricsHTTPHandler())
	httpComponent.AddHandler("/api/v0/version", netHTTP.HandlerFunc(func(w netHTTP.ResponseWriter, r *netHTTP.Request) {
		versionInfo := struct {
			Version   string `json:"version"`
			BuildDate string `json:"build_date"`
			Compiler  string `json:"compiler"`
		}{
			Version:   Version,
			BuildDate: BuildDate,
			Compiler:  runtime.Version(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(versionInfo)
	}))
	r.GaugeVec(reporter.GaugeOpts{
		Name: "info",
		Help: "Akvorado build information",
	}, []string{"version", "build_date", "compiler"}).
		WithLabelValues(Version, BuildDate, runtime.Version()).Set(1)

	// Start all the components.
	components := []interface{}{
		r,
		daemonComponent,
		httpComponent,
		flowComponent,
		snmpComponent,
		geoipComponent,
		kafkaComponent,
		clickhouseComponent,
		coreComponent,
		webComponent,
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
