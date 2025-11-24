// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"slices"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
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
	AutomaticRestart bool
	Reporting        reporter.Configuration
	HTTP             httpserver.Configuration
	ClickHouse       clickhouse.Configuration
	ClickHouseDB     clickhousedb.Configuration
	Kafka            kafka.Configuration
	GeoIP            geoip.Configuration
	Orchestrator     orchestrator.Configuration `mapstructure:",squash" yaml:",inline"`
	Schema           schema.Configuration
	// Other service configurations
	Inlet        []InletConfiguration        `validate:"dive"`
	Outlet       []OutletConfiguration       `validate:"dive"`
	Console      []ConsoleConfiguration      `validate:"dive"`
	DemoExporter []DemoExporterConfiguration `validate:"dive"`
}

// Reset resets the configuration of the orchestrator command to its default value.
func (c *OrchestratorConfiguration) Reset() {
	inletConfiguration := InletConfiguration{}
	inletConfiguration.Reset()
	outletConfiguration := OutletConfiguration{}
	outletConfiguration.Reset()
	consoleConfiguration := ConsoleConfiguration{}
	consoleConfiguration.Reset()
	*c = OrchestratorConfiguration{
		AutomaticRestart: true,
		Reporting:        reporter.DefaultConfiguration(),
		HTTP:             httpserver.DefaultConfiguration(),
		ClickHouse:       clickhouse.DefaultConfiguration(),
		ClickHouseDB:     clickhousedb.DefaultConfiguration(),
		Kafka:            kafka.DefaultConfiguration(),
		GeoIP:            geoip.DefaultConfiguration(),
		Orchestrator:     orchestrator.DefaultConfiguration(),
		Schema:           schema.DefaultConfiguration(),
		// Other service configurations
		Inlet:        []InletConfiguration{inletConfiguration},
		Outlet:       []OutletConfiguration{outletConfiguration},
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
	Long: `Akvorado is a NetFlow/IPFIX collector. The orchestrator service configures external
components and centralizes configuration of the various other components.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
	restart:
		config := OrchestratorConfiguration{}
		OrchestratorOptions.Path = args[0]
		OrchestratorOptions.BeforeDump = func(metadata mapstructure.Metadata) {
			// Override some parts of the configuration
			for idx := range config.Inlet {
				if !slices.Contains(metadata.Keys, fmt.Sprintf("Inlet[%d].Kafka.Brokers[0]", idx)) {
					config.Inlet[idx].Kafka.Configuration = config.Kafka.Configuration
				}
			}
			for idx := range config.Outlet {
				if !slices.Contains(metadata.Keys, fmt.Sprintf("Outlet[%d].ClickHouse.Servers[0]", idx)) {
					config.Outlet[idx].ClickHouseDB = config.ClickHouseDB
				}
				if !slices.Contains(metadata.Keys, fmt.Sprintf("Outlet[%d].Kafka.Brokers[0]", idx)) {
					config.Outlet[idx].Kafka.Configuration = config.Kafka.Configuration
				}
				config.Outlet[idx].Schema = config.Schema
			}
			for idx := range config.Console {
				if !slices.Contains(metadata.Keys, fmt.Sprintf("Console[%d].ClickHouse.Servers[0]", idx)) {
					config.Console[idx].ClickHouse = config.ClickHouseDB
				}
				config.Console[idx].Schema = config.Schema
			}
		}
		// Parse and check the configuration a first time to start monitoring
		// file changes if automatic restart is enabled.
		paths, err := OrchestratorOptions.Parse(cmd.OutOrStdout(), "orchestrator", &config)
		if err != nil {
			return err
		}
		OrchestratorOptions.Dump = false

		// Start a few of the components we need for automatic restart
		r, err := reporter.New(config.Reporting)
		if err != nil {
			return fmt.Errorf("unable to initialize reporter: %w", err)
		}
		daemonComponent, err := daemon.New(r)
		if err != nil {
			return fmt.Errorf("unable to initialize daemon component: %w", err)
		}

		configurationModified := atomic.Bool{}
		if config.AutomaticRestart && !OrchestratorOptions.CheckMode {
			orchestratorWatch(r, daemonComponent, paths, &configurationModified)
		}

		// Then, a second time for the remaining of the configuration.
		config = OrchestratorConfiguration{}
		if _, err := OrchestratorOptions.Parse(cmd.OutOrStdout(), "orchestrator", &config); err != nil {
			return err
		}

		if err := orchestratorStart(r, config, daemonComponent, OrchestratorOptions.CheckMode); err != nil {
			return err
		}
		if configurationModified.Load() {
			goto restart
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(orchestratorCmd)
	orchestratorCmd.Flags().BoolVarP(&OrchestratorOptions.ConfigRelatedOptions.Dump, "dump", "D", false,
		"Dump configuration before starting")
	orchestratorCmd.Flags().BoolVarP(&OrchestratorOptions.CheckMode, "check", "C", false,
		"Check configuration, but does not start")
}

func orchestratorStart(r *reporter.Reporter, config OrchestratorConfiguration, daemonComponent daemon.Component, checkOnly bool) error {
	httpComponent, err := httpserver.New(r, "orchestrator", config.HTTP, httpserver.Dependencies{
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
	for idx := range config.Outlet {
		orchestratorComponent.RegisterConfiguration(orchestrator.OutletService, config.Outlet[idx])
	}
	for idx := range config.Console {
		orchestratorComponent.RegisterConfiguration(orchestrator.ConsoleService, config.Console[idx])
	}
	for idx := range config.DemoExporter {
		orchestratorComponent.RegisterConfiguration(orchestrator.DemoExporterService, config.DemoExporter[idx])
	}

	// Expose some information and metrics
	addCommonHTTPHandlers(r, "orchestrator", httpComponent)
	moreMetrics(r)

	// If we only asked for a check, stop here.
	if checkOnly {
		return nil
	}

	// Start all the components.
	components := []any{
		geoipComponent,
		httpComponent,
		clickhouseDBComponent,
		clickhouseComponent,
		kafkaComponent,
	}
	return StartStopComponents(r, daemonComponent, components)
}

// orchestratorWatch will listen to changes to the given path and trigger a
// restart of the orchestrator if any. When a modification is detected, the
// modified chan is closed. The internal goroutine is also stopped if there the
// modified chan is closed.
func orchestratorWatch(r *reporter.Reporter, daemonComponent daemon.Component, paths []string, modified *atomic.Bool) {
	r.Info().Strs("paths", paths).Msg("watching loaded configuration files for changes")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		r.Err(err).Msg("cannot setup watcher for configuration changes")
		return
	}
	for _, path := range paths {
		if err := watcher.Add(filepath.Dir(path)); err != nil {
			r.Err(err).Msg("cannot watch configuration file")
			watcher.Close()
			return
		}
	}
	go func() {
		defer watcher.Close()
		for {
			select {
			case err, ok := <-watcher.Errors:
				if !ok {
					r.Error().Msg("configuration file watcher died")
				}
				r.Err(err).Msg("error from configuration file watcher")
				return
			case event, ok := <-watcher.Events:
				if !ok {
					r.Error().Msg("configuration file watcher died")
					return
				}
				if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
					// Check if we have one of the monitored path matching
					r.Debug().Str("name", event.Name).Msg("detected potential configuration change")
					if !slices.Contains(paths, filepath.Clean(event.Name)) {
						continue
					}

					// Check if the configuration is correct
					_, err = OrchestratorOptions.Parse(io.Discard, "orchestrator", &OrchestratorConfiguration{})
					if err != nil {
						r.Err(err).Msg("cannot validate new configuration, not reloading")
						continue
					}

					// Request termination to reexec
					r.Info().Str("path", event.Name).Msg("restart on configuration change")
					modified.Store(true)
					daemonComponent.Terminate()
					return
				}
			case <-daemonComponent.Terminated():
				return
			}
		}
	}()
}

// orchestratorGeoIPMigrationHook migrates GeoIP configuration from inlet
// component to clickhouse component
func orchestratorGeoIPMigrationHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (any, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeFor[OrchestratorConfiguration]() {
			return from.Interface(), nil
		}

		var inletKey, geoIPKey, inletGeoIPValue *reflect.Value

		fromKeys := from.MapKeys()
		for i, k := range fromKeys {
			k = helpers.ElemOrIdentity(k)
			if k.Kind() != reflect.String {
				return from.Interface(), nil
			}
			if helpers.MapStructureMatchName(k.String(), "Inlet") {
				inletKey = &fromKeys[i]
			} else if helpers.MapStructureMatchName(k.String(), "GeoIP") {
				geoIPKey = &fromKeys[i]
			}
		}
		if inletKey == nil {
			return from.Interface(), nil
		}

		// Take the first geoip configuration and delete the others
		inletConfigs := helpers.ElemOrIdentity(from.MapIndex(*inletKey))
		if inletConfigs.Kind() != reflect.Slice {
			inletConfigs = reflect.ValueOf([]any{inletConfigs.Interface()})
		}
		for i := range inletConfigs.Len() {
			fromInlet := helpers.ElemOrIdentity(inletConfigs.Index(i))
			if fromInlet.Kind() != reflect.Map {
				return from.Interface(), nil
			}
			fromInletKeys := fromInlet.MapKeys()
			for _, k := range fromInletKeys {
				k = helpers.ElemOrIdentity(k)
				if k.Kind() != reflect.String {
					return from.Interface(), nil
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
			return from.Interface(), nil
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

		return from.Interface(), nil
	}
}

// orchestratorClickHouseMigrationHook migrates ClickHouse database
// configuration from clickhouse component to clickhousedb component
func orchestratorClickHouseMigrationHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (any, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeFor[OrchestratorConfiguration]() {
			return from.Interface(), nil
		}

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

		return from.Interface(), nil
	}
}

// orchestratorInletToOutletMigrationHook migrates inlet configuration to outlet
// configuration. This only works if there is no outlet configuration and if
// there is only one inlet configuration.
func orchestratorInletToOutletMigrationHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (any, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeFor[OrchestratorConfiguration]() {
			return from.Interface(), nil
		}

		// inlet fields (Metadata, Routing, Core, Schema) → outlet
		var inletKey, outletKey *reflect.Value
		fromKeys := from.MapKeys()
		for i, k := range fromKeys {
			k = helpers.ElemOrIdentity(k)
			if k.Kind() != reflect.String {
				continue
			}
			if helpers.MapStructureMatchName(k.String(), "Inlet") {
				inletKey = &fromKeys[i]
			} else if helpers.MapStructureMatchName(k.String(), "Outlet") {
				outletKey = &fromKeys[i]
			}
		}

		if inletKey != nil {
			inletConfigs := helpers.ElemOrIdentity(from.MapIndex(*inletKey))
			if inletConfigs.Kind() != reflect.Slice {
				inletConfigs = reflect.ValueOf([]any{inletConfigs.Interface()})
			}

			// Fields to migrate from inlet to outlet
			fieldsToMigrate := []string{
				// Current keys
				"Metadata", "Routing", "Core", "Schema",
				// Older keys (which will be migrated)
				"BMP", "SNMP",
			}

			// Process each inlet configuration
			for i := range inletConfigs.Len() {
				fromInlet := helpers.ElemOrIdentity(inletConfigs.Index(i))
				if fromInlet.Kind() != reflect.Map {
					continue
				}
				modified := false
				toOutlet := reflect.ValueOf(gin.H{})

				// Migrate fields from inlet to outlet
				fromInletKeys := fromInlet.MapKeys()
				for _, k := range fromInletKeys {
					k = helpers.ElemOrIdentity(k)
					if k.Kind() != reflect.String {
						continue
					}
					for _, field := range fieldsToMigrate {
						if !helpers.MapStructureMatchName(k.String(), field) {
							continue
						}
						// We can only do a migration if we have no existing
						// outlet configuration AND only one inlet configuration.
						if outletKey != nil {
							return nil, fmt.Errorf("cannot have both \"inlet\" configuration with %q field and \"outlet\" configuration", field)
						}
						if inletConfigs.Len() > 1 {
							return nil, fmt.Errorf("cannot migrate %q from %q to %q as there are several inlet configurations", field, "inlet", "outlet")
						}
						toOutlet.SetMapIndex(k, helpers.ElemOrIdentity(fromInlet.MapIndex(k)))
						fromInlet.SetMapIndex(k, reflect.Value{})
						modified = true
						break
					}
				}

				if modified {
					// We know there is no existing outlet configuration.
					outletConfigs := reflect.ValueOf([]any{toOutlet})
					from.SetMapIndex(reflect.ValueOf("outlet"), outletConfigs)
				}
			}
		}

		return from.Interface(), nil
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(orchestratorGeoIPMigrationHook())
	helpers.RegisterMapstructureUnmarshallerHook(orchestratorClickHouseMigrationHook())
	helpers.RegisterMapstructureUnmarshallerHook(orchestratorInletToOutletMigrationHook())
}
