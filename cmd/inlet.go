// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/bmp"
	"akvorado/inlet/core"
	"akvorado/inlet/flow"
	"akvorado/inlet/geoip"
	"akvorado/inlet/kafka"
	"akvorado/inlet/metadata"
	"akvorado/inlet/metadata/provider/snmp"
)

// InletConfiguration represents the configuration file for the inlet command.
type InletConfiguration struct {
	Reporting reporter.Configuration
	HTTP      httpserver.Configuration
	Flow      flow.Configuration
	Metadata  metadata.Configuration
	BMP       bmp.Configuration
	GeoIP     geoip.Configuration
	Kafka     kafka.Configuration
	Core      core.Configuration
	Schema    schema.Configuration
}

// Reset resets the configuration for the inlet command to its default value.
func (c *InletConfiguration) Reset() {
	*c = InletConfiguration{
		HTTP:      httpserver.DefaultConfiguration(),
		Reporting: reporter.DefaultConfiguration(),
		Flow:      flow.DefaultConfiguration(),
		Metadata:  metadata.DefaultConfiguration(),
		BMP:       bmp.DefaultConfiguration(),
		GeoIP:     geoip.DefaultConfiguration(),
		Kafka:     kafka.DefaultConfiguration(),
		Core:      core.DefaultConfiguration(),
		Schema:    schema.DefaultConfiguration(),
	}
	c.Metadata.Provider.Config = snmp.DefaultConfiguration()
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
	flowComponent, err := flow.New(r, config.Flow, flow.Dependencies{
		Daemon: daemonComponent,
		HTTP:   httpComponent,
		Schema: schemaComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize flow component: %w", err)
	}
	metadataComponent, err := metadata.New(r, config.Metadata, metadata.Dependencies{
		Daemon: daemonComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize METADATA component: %w", err)
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
		Schema: schemaComponent,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize Kafka component: %w", err)
	}
	coreComponent, err := core.New(r, config.Core, core.Dependencies{
		Daemon:   daemonComponent,
		Flow:     flowComponent,
		Metadata: metadataComponent,
		BMP:      bmpComponent,
		GeoIP:    geoipComponent,
		Kafka:    kafkaComponent,
		HTTP:     httpComponent,
		Schema:   schemaComponent,
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
		metadataComponent,
		bmpComponent,
		geoipComponent,
		kafkaComponent,
		coreComponent,
		flowComponent,
	}
	return StartStopComponents(r, daemonComponent, components)
}

// InletConfigurationUnmarshallerHook renames SNMP configuration to metadata
func InletConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeOf(InletConfiguration{}) {
			return from.Interface(), nil
		}

		// snmp → metadata
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
		if snmpKey == nil {
			return from.Interface(), nil
		}
		if snmpKey != nil && metadataKey != nil {
			return nil, fmt.Errorf("cannot have both %q and %q", snmpKey.String(), metadataKey.String())
		}

		// Build the metadata configuration
		providerValue := gin.H{}
		metadataValue := gin.H{}
		// Dispatch values from snmp key into metadata
		snmpMap := helpers.ElemOrIdentity(from.MapIndex(*snmpKey))
		snmpKeys := snmpMap.MapKeys()
	outer:
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
			for j := 0; j < metadataConfig.NumField(); j++ {
				if helpers.MapStructureMatchName(k.String(), metadataConfig.Field(j).Name) {
					metadataValue[k.String()] = snmpMap.MapIndex(snmpKeys[i]).Interface()
					continue outer
				}
			}
			providerValue[k.String()] = snmpMap.MapIndex(snmpKeys[i]).Interface()
		}

		providerValue["type"] = "snmp"
		metadataValue["provider"] = providerValue
		from.SetMapIndex(reflect.ValueOf("metadata"), reflect.ValueOf(metadataValue))
		from.SetMapIndex(*snmpKey, reflect.Value{})
		return from.Interface(), nil
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(InletConfigurationUnmarshallerHook())
}
