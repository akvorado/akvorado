// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/cobra"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/orchestrator"
	"akvorado/orchestrator/clickhouse"
	"akvorado/orchestrator/geoip"
	"akvorado/orchestrator/kafka"
)

// OrchestratorConfiguration represents the configuration file for the orchestrator command.
type OrchestratorConfiguration struct {
	Reporting    reporter.Configuration
	HTTP         httpserver.Configuration
	ClickHouse   clickhouse.Configuration
	ClickHouseDB clickhousedb.Configuration
	Kafka        kafka.Configuration
	GeoIP        geoip.Configuration
	Orchestrator orchestrator.Configuration `mapstructure:",squash" yaml:",inline"`
	Schema       schema.Configuration
	// Other service configurations
	Inlet        []InletConfiguration        `validate:"dive"`
	Console      []ConsoleConfiguration      `validate:"dive"`
	DemoExporter []DemoExporterConfiguration `validate:"dive"`
}

// Reset resets the configuration of the orchestrator command to its default value.
func (c *OrchestratorConfiguration) Reset() {
	inletConfiguration := InletConfiguration{}
	inletConfiguration.Reset()
	consoleConfiguration := ConsoleConfiguration{}
	consoleConfiguration.Reset()
	*c = OrchestratorConfiguration{
		Reporting:    reporter.DefaultConfiguration(),
		HTTP:         httpserver.DefaultConfiguration(),
		ClickHouse:   clickhouse.DefaultConfiguration(),
		ClickHouseDB: clickhousedb.DefaultConfiguration(),
		Kafka:        kafka.DefaultConfiguration(),
		GeoIP:        geoip.DefaultConfiguration(),
		Orchestrator: orchestrator.DefaultConfiguration(),
		Schema:       schema.DefaultConfiguration(),
		// Other service configurations
		Inlet:        []InletConfiguration{inletConfiguration},
		Console:      []ConsoleConfiguration{consoleConfiguration},
		DemoExporter: []DemoExporterConfiguration{},
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
		config := OrchestratorConfiguration{}
		OrchestratorOptions.Path = args[0]
		OrchestratorOptions.BeforeDump = func(metadata mapstructure.Metadata) {
			// Override some parts of the configuration
			if !slices.Contains(metadata.Keys, "ClickHouse.Kafka.Brokers[0]") {
				config.ClickHouse.Kafka.Configuration = config.Kafka.Configuration
			}
			for idx := range config.Inlet {
				if !slices.Contains(metadata.Keys, fmt.Sprintf("Inlet[%d].Kafka.Brokers[0]", idx)) {
					config.Inlet[idx].Kafka.Configuration = config.Kafka.Configuration
				}
				config.Inlet[idx].Schema = config.Schema
			}
			for idx := range config.Console {
				if !slices.Contains(metadata.Keys, fmt.Sprintf("Console[%d].ClickHouse.Servers[0]", idx)) {
					config.Console[idx].ClickHouse = config.ClickHouseDB
				}
				config.Console[idx].Schema = config.Schema
			}
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
	httpComponent, err := httpserver.New(r, config.HTTP, httpserver.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize HTTP component: %w", err)
	}
	schemaComponent, err := schema.New(config.Schema)
	if err != nil {
		return fmt.Errorf("unable to initialize schema component: %w", err)
	}
	kafkaComponent, err := kafka.New(r, config.Kafka, kafka.Dependencies{Schema: schemaComponent})
	if err != nil {
		return fmt.Errorf("unable to initialize kafka component: %w", err)
	}
	clickhouseDBComponent, err := clickhousedb.New(r, config.ClickHouseDB, clickhousedb.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize ClickHouse component: %w", err)
	}

	geoipComponent, err := geoip.New(r, config.GeoIP, geoip.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize GeoIP component: %w", err)
	}

	clickhouseComponent, err := clickhouse.New(r, config.ClickHouse, clickhouse.Dependencies{
		Daemon:     daemonComponent,
		HTTP:       httpComponent,
		ClickHouse: clickhouseDBComponent,
		Schema:     schemaComponent,
		GeoIP:      geoipComponent,
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
	for idx := range config.Inlet {
		orchestratorComponent.RegisterConfiguration(orchestrator.InletService, config.Inlet[idx])
	}
	for idx := range config.Console {
		orchestratorComponent.RegisterConfiguration(orchestrator.ConsoleService, config.Console[idx])
	}
	for idx := range config.DemoExporter {
		orchestratorComponent.RegisterConfiguration(orchestrator.DemoExporterService, config.DemoExporter[idx])
	}

	// Expose some information and metrics
	addCommonHTTPHandlers(r, "orchestrator", httpComponent)
	versionMetrics(r)

	// If we only asked for a check, stop here.
	if checkOnly {
		return nil
	}

	// Start all the components.
	components := []interface{}{
		geoipComponent,
		httpComponent,
		clickhouseDBComponent,
		clickhouseComponent,
		kafkaComponent,
	}
	return StartStopComponents(r, daemonComponent, components)
}

// OrchestratorConfigurationUnmarshallerHook migrates GeoIP configuration from inlet
// component to clickhouse component and ClickHouse database configuration from
// clickhouse component to clickhousedb component.
func OrchestratorConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeOf(OrchestratorConfiguration{}) {
			return from.Interface(), nil
		}

	inletgeoip:
		// inlet/geoip → geoip
		for {
			var (
				inletKey, geoIPKey, inletGeoIPValue *reflect.Value
			)

			fromKeys := from.MapKeys()
			for i, k := range fromKeys {
				k = helpers.ElemOrIdentity(k)
				if k.Kind() != reflect.String {
					break inletgeoip
				}
				if helpers.MapStructureMatchName(k.String(), "Inlet") {
					inletKey = &fromKeys[i]
				} else if helpers.MapStructureMatchName(k.String(), "GeoIP") {
					geoIPKey = &fromKeys[i]
				}
			}
			if inletKey == nil {
				break inletgeoip
			}

			// Take the first geoip configuration and delete the others
			inletConfigs := helpers.ElemOrIdentity(from.MapIndex(*inletKey))
			if inletConfigs.Kind() != reflect.Slice {
				inletConfigs = reflect.ValueOf([]interface{}{inletConfigs.Interface()})
			}
			for i := range inletConfigs.Len() {
				fromInlet := helpers.ElemOrIdentity(inletConfigs.Index(i))
				if fromInlet.Kind() != reflect.Map {
					break inletgeoip
				}
				fromInletKeys := fromInlet.MapKeys()
				for _, k := range fromInletKeys {
					k = helpers.ElemOrIdentity(k)
					if k.Kind() != reflect.String {
						break inletgeoip
					}
					if helpers.MapStructureMatchName(k.String(), "GeoIP") {
						if inletGeoIPValue == nil {
							v := fromInlet.MapIndex(k)
							inletGeoIPValue = &v
						}
					}
				}
			}
			if inletGeoIPValue == nil {
				break inletgeoip
			}
			if geoIPKey != nil {
				return nil, errors.New("cannot have both \"GeoIP\" in inlet and clickhouse configuration")
			}

			from.SetMapIndex(reflect.ValueOf("geoip"), *inletGeoIPValue)
			for i := range inletConfigs.Len() {
				fromInlet := helpers.ElemOrIdentity(inletConfigs.Index(i))
				fromInletKeys := fromInlet.MapKeys()
				for _, k := range fromInletKeys {
					k = helpers.ElemOrIdentity(k)
					if helpers.MapStructureMatchName(k.String(), "GeoIP") {
						fromInlet.SetMapIndex(k, reflect.Value{})
					}
				}
			}
			break
		}

		{
			// clickhouse database fields → clickhousedb
			var clickhouseKey, clickhouseDBKey *reflect.Value
			fromKeys := from.MapKeys()
			for i, k := range fromKeys {
				k = helpers.ElemOrIdentity(k)
				if k.Kind() != reflect.String {
					continue
				}
				if helpers.MapStructureMatchName(k.String(), "ClickHouse") {
					clickhouseKey = &fromKeys[i]
				} else if helpers.MapStructureMatchName(k.String(), "ClickHouseDB") {
					clickhouseDBKey = &fromKeys[i]
				}
			}

			if clickhouseKey != nil {
				var clickhouseDB reflect.Value
				if clickhouseDBKey != nil {
					clickhouseDB = helpers.ElemOrIdentity(from.MapIndex(*clickhouseDBKey))
				} else {
					clickhouseDB = reflect.ValueOf(gin.H{})
				}

				clickhouse := helpers.ElemOrIdentity(from.MapIndex(*clickhouseKey))
				if clickhouse.Kind() == reflect.Map {
					clickhouseKeys := clickhouse.MapKeys()
					// Fields to migrate from clickhouse to clickhousedb
					fieldsToMigrate := []string{
						"Servers", "Cluster", "Database", "Username", "Password",
						"MaxOpenConns", "DialTimeout", "TLS",
					}
					found := false
					for _, k := range clickhouseKeys {
						k = helpers.ElemOrIdentity(k)
						if k.Kind() != reflect.String {
							continue
						}
						for _, field := range fieldsToMigrate {
							if helpers.MapStructureMatchName(k.String(), field) {
								if clickhouseDBKey != nil {
									return nil, errors.New("cannot have both \"ClickHouseDB\" and ClickHouse database settings in \"ClickHouse\"")
								}
								clickhouseDB.SetMapIndex(k, helpers.ElemOrIdentity(clickhouse.MapIndex(k)))
								clickhouse.SetMapIndex(k, reflect.Value{})
								found = true
								break
							}
						}
					}
					if clickhouseDBKey == nil && found {
						from.SetMapIndex(reflect.ValueOf("clickhousedb"), clickhouseDB)
					}
				}
			}
		}

		return from.Interface(), nil
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(OrchestratorConfigurationUnmarshallerHook())
}
