// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/cobra"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/clickhouse"
	"akvorado/outlet/core"
	"akvorado/outlet/flow"
	"akvorado/outlet/kafka"
	"akvorado/outlet/metadata"
	"akvorado/outlet/metadata/provider/snmp"
	"akvorado/outlet/routing"
	"akvorado/outlet/routing/provider/bmp"
)

// OutletConfiguration represents the configuration file for the outlet command.
type OutletConfiguration struct {
	Reporting    reporter.Configuration
	HTTP         httpserver.Configuration
	Metadata     metadata.Configuration
	Routing      routing.Configuration
	Kafka        kafka.Configuration
	ClickHouseDB clickhousedb.Configuration
	ClickHouse   clickhouse.Configuration
	Core         core.Configuration
	Schema       schema.Configuration
}

// Reset resets the configuration for the outlet command to its default value.
func (c *OutletConfiguration) Reset() {
	*c = OutletConfiguration{
		HTTP:         httpserver.DefaultConfiguration(),
		Reporting:    reporter.DefaultConfiguration(),
		Metadata:     metadata.DefaultConfiguration(),
		Routing:      routing.DefaultConfiguration(),
		Kafka:        kafka.DefaultConfiguration(),
		ClickHouseDB: clickhousedb.DefaultConfiguration(),
		ClickHouse:   clickhouse.DefaultConfiguration(),
		Core:         core.DefaultConfiguration(),
		Schema:       schema.DefaultConfiguration(),
	}
	c.Metadata.Providers = []metadata.ProviderConfiguration{{Config: snmp.DefaultConfiguration()}}
	c.Routing.Provider.Config = bmp.DefaultConfiguration()
}

type outletOptions struct {
	ConfigRelatedOptions
	CheckMode bool
}

// OutletOptions stores the command-line option values for the outlet
// command.
var OutletOptions outletOptions

var outletCmd = &cobra.Command{
	Use:   "outlet",
	Short: "Start Akvorado's outlet service",
	Long: `Akvorado is a Netflow/IPFIX collector. The outlet service handles flow ingestion,
enrichment and export to Kafka.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		config := OutletConfiguration{}
		OutletOptions.Path = args[0]
		if err := OutletOptions.Parse(cmd.OutOrStdout(), "outlet", &config); err != nil {
			return err
		}

		r, err := reporter.New(config.Reporting)
		if err != nil {
			return fmt.Errorf("unable to initialize reporter: %w", err)
		}
		return outletStart(r, config, OutletOptions.CheckMode)
	},
}

func init() {
	RootCmd.AddCommand(outletCmd)
	outletCmd.Flags().BoolVarP(&OutletOptions.ConfigRelatedOptions.Dump, "dump", "D", false,
		"Dump configuration before starting")
	outletCmd.Flags().BoolVarP(&OutletOptions.CheckMode, "check", "C", false,
		"Check configuration, but does not start")
}

func outletStart(r *reporter.Reporter, config OutletConfiguration, checkOnly bool) error {
	// Initialize the various components
	daemonComponent, err := daemon.New(r)
	if err != nil {
		return fmt.Errorf("unable to initialize daemon component: %w", err)
	}
	httpComponent, err := httpserver.New(r, config.HTTP, httpserver.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize http component: %w", err)
	}
	schemaComponent, err := schema.New(config.Schema)
	if err != nil {
		return fmt.Errorf("unable to initialize schema component: %w", err)
	}
	flowComponent, err := flow.New(r, flow.Dependencies{
		Schema: schemaComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize flow component: %w", err)
	}
	metadataComponent, err := metadata.New(r, config.Metadata, metadata.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize metadata component: %w", err)
	}
	routingComponent, err := routing.New(r, config.Routing, routing.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize routing component: %w", err)
	}
	kafkaComponent, err := kafka.New(r, config.Kafka, kafka.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize Kafka component: %w", err)
	}
	clickhouseDBComponent, err := clickhousedb.New(r, config.ClickHouseDB, clickhousedb.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize ClickHouse component: %w", err)
	}
	clickhouseComponent, err := clickhouse.New(r, config.ClickHouse, clickhouse.Dependencies{
		ClickHouse: clickhouseDBComponent,
		Schema:     schemaComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize outlet ClickHouse component: %w", err)
	}
	coreComponent, err := core.New(r, config.Core, core.Dependencies{
		Daemon:     daemonComponent,
		Flow:       flowComponent,
		Metadata:   metadataComponent,
		Routing:    routingComponent,
		Kafka:      kafkaComponent,
		ClickHouse: clickhouseComponent,
		HTTP:       httpComponent,
		Schema:     schemaComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize core component: %w", err)
	}

	// Expose some information and metrics
	addCommonHTTPHandlers(r, "outlet", httpComponent)
	versionMetrics(r)

	// If we only asked for a check, stop here.
	if checkOnly {
		return nil
	}

	// Start all the components.
	components := []any{
		httpComponent,
		clickhouseDBComponent,
		clickhouseComponent,
		flowComponent,
		metadataComponent,
		routingComponent,
		kafkaComponent,
		coreComponent,
	}
	return StartStopComponents(r, daemonComponent, components)
}

// OutletConfigurationUnmarshallerHook renames SNMP configuration to metadata and
// BMP configuration to routing.
func OutletConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeOf(OutletConfiguration{}) {
			return from.Interface(), nil
		}

		// snmp → metadata
		{
			var snmpKey, metadataKey *reflect.Value
			fromKeys := from.MapKeys()
			for i, k := range fromKeys {
				k = helpers.ElemOrIdentity(k)
				if k.Kind() != reflect.String {
					return from.Interface(), nil
				}
				if helpers.MapStructureMatchName(k.String(), "Snmp") {
					snmpKey = &fromKeys[i]
				} else if helpers.MapStructureMatchName(k.String(), "Metadata") {
					metadataKey = &fromKeys[i]
				}
			}
			if snmpKey != nil {
				if metadataKey != nil {
					return nil, fmt.Errorf("cannot have both %q and %q", snmpKey.String(), metadataKey.String())
				}

				// Build the metadata configuration
				providerValue := gin.H{}
				metadataValue := gin.H{}
				// Dispatch values from snmp key into metadata
				snmpMap := helpers.ElemOrIdentity(from.MapIndex(*snmpKey))
				snmpKeys := snmpMap.MapKeys()
			outerSNMP:
				for i, k := range snmpKeys {
					k = helpers.ElemOrIdentity(k)
					if k.Kind() != reflect.String {
						continue
					}
					if helpers.MapStructureMatchName(k.String(), "PollerCoalesce") {
						metadataValue["MaxBatchRequests"] = snmpMap.MapIndex(snmpKeys[i]).Interface()
						continue
					}
					metadataConfig := reflect.TypeOf(metadata.Configuration{})
					for j := range metadataConfig.NumField() {
						if helpers.MapStructureMatchName(k.String(), metadataConfig.Field(j).Name) {
							metadataValue[k.String()] = snmpMap.MapIndex(snmpKeys[i]).Interface()
							continue outerSNMP
						}
					}
					providerValue[k.String()] = snmpMap.MapIndex(snmpKeys[i]).Interface()
				}

				providerValue["type"] = "snmp"
				metadataValue["provider"] = providerValue
				from.SetMapIndex(reflect.ValueOf("metadata"), reflect.ValueOf(metadataValue))
				from.SetMapIndex(*snmpKey, reflect.Value{})
			}
		}

		// bmp → routing
		{
			var bmpKey, routingKey *reflect.Value
			fromKeys := from.MapKeys()
			for i, k := range fromKeys {
				k = helpers.ElemOrIdentity(k)
				if k.Kind() != reflect.String {
					return from.Interface(), nil
				}
				if helpers.MapStructureMatchName(k.String(), "Bmp") {
					bmpKey = &fromKeys[i]
				} else if helpers.MapStructureMatchName(k.String(), "Routing") {
					routingKey = &fromKeys[i]
				}
			}
			if bmpKey != nil {
				if routingKey != nil {
					return nil, fmt.Errorf("cannot have both %q and %q", bmpKey.String(), routingKey.String())
				}

				// Build the routing configuration
				providerValue := gin.H{}
				routingValue := gin.H{}
				// Dispatch values from bmp key into routing
				bmpMap := helpers.ElemOrIdentity(from.MapIndex(*bmpKey))
				bmpKeys := bmpMap.MapKeys()
			outerBMP:
				for i, k := range bmpKeys {
					k = helpers.ElemOrIdentity(k)
					if k.Kind() != reflect.String {
						continue
					}
					routingConfig := reflect.TypeOf(routing.Configuration{})
					for j := range routingConfig.NumField() {
						if helpers.MapStructureMatchName(k.String(), routingConfig.Field(j).Name) {
							routingValue[k.String()] = bmpMap.MapIndex(bmpKeys[i]).Interface()
							continue outerBMP
						}
					}
					providerValue[k.String()] = bmpMap.MapIndex(bmpKeys[i]).Interface()
				}

				providerValue["type"] = "bmp"
				routingValue["provider"] = providerValue
				from.SetMapIndex(reflect.ValueOf("routing"), reflect.ValueOf(routingValue))
				from.SetMapIndex(*bmpKey, reflect.Value{})
			}
		}

		return from.Interface(), nil
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(OutletConfigurationUnmarshallerHook())
}
