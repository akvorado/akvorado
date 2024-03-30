// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gin-gonic/gin"
	"golang.org/x/exp/slices"

	"akvorado/common/schema"
)

var errSkipStep = errors.New("migration: skip this step")

// wrapMigrations can be used to wrap migration functions. It will keep the
// metrics up-to-date as long as the migration function returns `errSkipStep`
// when a step is skipped.
func (c *Component) wrapMigrations(ctx context.Context, fns ...func(context.Context) error) error {
	for _, fn := range fns {
		if err := fn(ctx); err == nil {
			c.metrics.migrationsApplied.Inc()
		} else if err == errSkipStep {
			c.metrics.migrationsNotApplied.Inc()
		} else {
			return err
		}
	}
	return nil
}

// stemplate is a simple wrapper around text/template.
func stemplate(t string, data any) (string, error) {
	tpl, err := template.New("tpl").Option("missingkey=error").Parse(t)
	if err != nil {
		return "", err
	}
	var result strings.Builder
	if err := tpl.Execute(&result, data); err != nil {
		return "", err
	}
	return result.String(), nil
}

// tableAlreadyExists compare the provided table with the one in database.
// `column` can either be "create_table_query" or "as_select". target is the
// expected value.
func (c *Component) tableAlreadyExists(ctx context.Context, table, column, target string) (bool, error) {
	// Normalize a bit the target. This is far from perfect, but we test that
	// and we hope this does not differ between ClickHouse versions!
	target = strings.TrimSpace(regexp.MustCompile("\\s+").ReplaceAllString(target, " "))

	// Fetch the existing one
	row := c.d.ClickHouse.QueryRow(ctx,
		fmt.Sprintf("SELECT %s FROM system.tables WHERE name = $1 AND database = $2", column),
		table, c.config.Database)
	var existing string
	if err := row.Scan(&existing); err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("cannot check if table %s already exists: %w", table, err)
	}
	// Add a few tweaks
	existing = strings.ReplaceAll(existing,
		fmt.Sprintf(`dictGetOrDefault('%s.`, c.config.Database),
		"dictGetOrDefault('")
	existing = strings.ReplaceAll(existing,
		fmt.Sprintf(`dictGet('%s.`, c.config.Database),
		"dictGet('")
	existing = regexp.MustCompile(` SETTINGS index_granularity = \d+$`).ReplaceAllString(existing, "")

	// Compare!
	if existing == target {
		return true, nil
	}
	c.r.Debug().
		Str("target", target).Str("existing", existing).
		Msgf("table %s state difference detected", table)
	return false, nil
}

// mergeTreeEngine returns a MergeTree engine definition, either plain or using
// Replicated if we are on a cluster.
func (c *Component) mergeTreeEngine(table string, variant string, args ...string) string {
	if c.config.Cluster != "" {
		return fmt.Sprintf(`Replicated%sMergeTree(%s)`, variant, strings.Join(
			append([]string{
				fmt.Sprintf("'/clickhouse/tables/shard-{shard}/%s'", table),
				"'replica-{replica}'",
			}, args...),
			", "))
	}
	if len(args) == 0 {
		return fmt.Sprintf("%sMergeTree", variant)
	}
	return fmt.Sprintf("%sMergeTree(%s)", variant, strings.Join(args, ", "))
}

// distributedTable turns a table name to the matching distributed table if we
// are in a cluster.
func (c *Component) distributedTable(table string) string {
	return table
}

// localTable turns a table name to the matching local distributed table if we
// are in a cluster.
func (c *Component) localTable(table string) string {
	if c.config.Cluster != "" {
		return fmt.Sprintf("%s_local", table)
	}
	return table
}

// createDictionary creates the provided dictionary.
func (c *Component) createDictionary(ctx context.Context, name, layout, schema, primary string) error {
	url := fmt.Sprintf("%s/api/v0/orchestrator/clickhouse/%s.csv", c.config.OrchestratorURL, name)
	source := fmt.Sprintf(`SOURCE(HTTP(URL '%s' FORMAT 'CSVWithNames'))`, url)
	settings := `SETTINGS(format_csv_allow_single_quotes = 0)`
	createQuery, err := stemplate(`
CREATE DICTIONARY {{ .Database }}.{{ .Name }} ({{ .Schema }})
PRIMARY KEY {{ .PrimaryKey}}
{{ .Source }}
LIFETIME(MIN 0 MAX 3600)
LAYOUT({{ .Layout }}())
{{ .Settings }}
`, gin.H{
		"Database":   c.config.Database,
		"Name":       name,
		"Schema":     schema,
		"PrimaryKey": primary,
		"Layout":     strings.ToUpper(layout),
		"Source":     source,
		"Settings":   settings,
	})
	if err != nil {
		return fmt.Errorf("cannot build query to create dictionary %s: %w", name, err)
	}

	// Check if dictionary exists and create it if not
	if ok, err := c.tableAlreadyExists(ctx, name, "create_table_query", createQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msgf("dictionary %s already exists, skip migration", name)
		return errSkipStep
	}
	c.r.Info().Msgf("create dictionary %s", name)
	createOrReplaceQuery := strings.Replace(createQuery, "CREATE ", "CREATE OR REPLACE ", 1)
	if err := c.d.ClickHouse.ExecOnCluster(ctx, createOrReplaceQuery); err != nil {
		return fmt.Errorf("cannot create dictionary %s: %w", name, err)
	}
	return nil
}

// createExportersTable creates the exporters table. This table is always local.
func (c *Component) createExportersTable(ctx context.Context) error {
	// Select the columns we need
	cols := []string{}
	for _, column := range c.d.Schema.Columns() {
		if column.Key == schema.ColumnTimeReceived || strings.HasPrefix(column.Name, "Exporter") {
			cols = append(cols, fmt.Sprintf("`%s` %s", column.Name, column.ClickHouseType))
		}
		if strings.HasPrefix(column.Name, "InIf") {
			cols = append(cols, fmt.Sprintf("`%s` %s",
				column.Name[2:], column.ClickHouseType,
			))
		}
	}

	// Build CREATE TABLE
	name := "exporters"
	createQuery, err := stemplate(
		`CREATE TABLE {{ .Database }}.{{ .Table }}
({{ .Schema }})
ENGINE = {{ .Engine }}
ORDER BY (ExporterAddress, IfName)
TTL TimeReceived + toIntervalDay(1)`,
		gin.H{
			"Database": c.config.Database,
			"Table":    name,
			"Schema":   strings.Join(cols, ", "),
			"Engine":   c.mergeTreeEngine(name, "Replacing", "TimeReceived"),
		})
	if err != nil {
		return fmt.Errorf("cannot build query to create exporters view: %w", err)
	}

	// Check if the table already exists
	if ok, err := c.tableAlreadyExists(ctx, name, "create_table_query", createQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msg("exporters table already exists, skip migration")
		return errSkipStep
	}

	// Drop existing table and recreate
	c.r.Info().Msg("create exporters table")
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
		"allow_suspicious_low_cardinality_types": 1,
	}))
	createOrReplaceQuery := strings.Replace(createQuery, "CREATE ", "CREATE OR REPLACE ", 1)
	if err := c.d.ClickHouse.ExecOnCluster(ctx, createOrReplaceQuery); err != nil {
		return fmt.Errorf("cannot create exporters table: %w", err)
	}

	return nil
}

// createExportersConsumerView creates the exporters view.
func (c *Component) createExportersConsumerView(ctx context.Context) error {
	// Select the columns we need
	cols := []string{}
	for _, column := range c.d.Schema.Columns() {
		if column.Key == schema.ColumnTimeReceived || strings.HasPrefix(column.Name, "Exporter") {
			cols = append(cols, column.Name)
		}
		if strings.HasPrefix(column.Name, "InIf") {
			cols = append(cols, fmt.Sprintf("[InIf%s, OutIf%s][num] AS If%s",
				column.Name[4:], column.Name[4:], column.Name[4:],
			))
		}
	}

	// Build SELECT query
	selectQuery, err := stemplate(
		`SELECT DISTINCT {{ .Columns }} FROM {{ .Database }}.{{ .Table }} ARRAY JOIN arrayEnumerate([1, 2]) AS num`,
		gin.H{
			"Table":    c.distributedTable("flows"),
			"Database": c.config.Database,
			"Columns":  strings.Join(cols, ", "),
		})
	if err != nil {
		return fmt.Errorf("cannot build query to create exporters view: %w", err)
	}

	// Check if the table already exists with these columns and with a TTL.
	if ok, err := c.tableAlreadyExists(ctx,
		"exporters_consumer", "as_select",
		selectQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msg("exporters view already exists, skip migration")
		return errSkipStep
	}

	// Drop existing table and recreate
	c.r.Info().Msg("create exporters view")
	if err := c.d.ClickHouse.ExecOnCluster(ctx, `DROP TABLE IF EXISTS exporters_consumer SYNC`); err != nil {
		return fmt.Errorf("cannot drop existing exporters view: %w", err)
	}
	if err := c.d.ClickHouse.ExecOnCluster(ctx, fmt.Sprintf(`
CREATE MATERIALIZED VIEW exporters_consumer TO %s AS %s
`, "exporters", selectQuery)); err != nil {
		return fmt.Errorf("cannot create exporters view: %w", err)
	}

	return nil
}

// createRawFlowsTable creates the raw flow table
func (c *Component) createRawFlowsTable(ctx context.Context) error {
	hash := c.d.Schema.ProtobufMessageHash()
	tableName := fmt.Sprintf("flows_%s_raw", hash)
	kafkaSettings := []string{
		fmt.Sprintf(`kafka_broker_list = '%s'`,
			strings.Join(c.config.Kafka.Brokers, ",")),
		fmt.Sprintf(`kafka_topic_list = '%s-%s'`,
			c.config.Kafka.Topic, hash),
		fmt.Sprintf(`kafka_group_name = '%s'`, c.config.Kafka.GroupName),
		`kafka_format = 'Protobuf'`,
		fmt.Sprintf(`kafka_schema = 'flow-%s.proto:FlowMessagev%s'`, hash, hash),
		fmt.Sprintf(`kafka_num_consumers = %d`, c.config.Kafka.Consumers),
		`kafka_thread_per_consumer = 1`,
		`kafka_handle_error_mode = 'stream'`,
	}
	for _, setting := range c.config.Kafka.EngineSettings {
		kafkaSettings = append(kafkaSettings, setting)
	}
	kafkaEngine := fmt.Sprintf("Kafka SETTINGS %s", strings.Join(kafkaSettings, ", "))

	// Build CREATE query
	createQuery, err := stemplate(
		`CREATE TABLE {{ .Database }}.{{ .Table }} ({{ .Schema }}) ENGINE = {{ .Engine }}`,
		gin.H{
			"Database": c.config.Database,
			"Table":    tableName,
			"Schema": c.d.Schema.ClickHouseCreateTable(
				schema.ClickHouseSkipGeneratedColumns,
				schema.ClickHouseUseTransformFromType,
				schema.ClickHouseSkipAliasedColumns),
			"Engine": kafkaEngine,
		})
	if err != nil {
		return fmt.Errorf("cannot build query to create raw flows table: %w", err)
	}

	// Check if the table already exists with the right schema
	if ok, err := c.tableAlreadyExists(ctx, tableName, "create_table_query", createQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msg("raw flows table already exists, skip migration")
		return errSkipStep
	}

	// Drop table if it exists as well as all the dependents and recreate the raw table
	c.r.Info().Msg("create raw flows table")
	for _, table := range []string{
		fmt.Sprintf("%s_consumer", tableName),
		fmt.Sprintf("%s_errors", tableName),
		tableName,
	} {
		if err := c.d.ClickHouse.ExecOnCluster(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s SYNC`, table)); err != nil {
			return fmt.Errorf("cannot drop %s: %w", table, err)
		}
	}
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
		"allow_suspicious_low_cardinality_types": 1,
	}))
	if err := c.d.ClickHouse.ExecOnCluster(ctx, createQuery); err != nil {
		return fmt.Errorf("cannot create raw flows table: %w", err)
	}

	return nil
}

var dictionaryNetworksLookupRegex = regexp.MustCompile(`\bc_(Src|Dst)Networks\[([[:lower:]]+)\]\B`)

func (c *Component) createRawFlowsConsumerView(ctx context.Context) error {
	tableName := fmt.Sprintf("flows_%s_raw", c.d.Schema.ProtobufMessageHash())
	viewName := fmt.Sprintf("%s_consumer", tableName)

	// Build SELECT query
	args := gin.H{
		"Columns": strings.Join(c.d.Schema.ClickHouseSelectColumns(
			schema.ClickHouseSubstituteGenerates,
			schema.ClickHouseSubstituteTransforms,
			schema.ClickHouseSkipAliasedColumns), ", "),
		"Database": c.config.Database,
		"Table":    tableName,
	}
	selectQuery, err := stemplate(
		`SELECT {{ .Columns }} FROM {{ .Database }}.{{ .Table }} WHERE length(_error) = 0`,
		args)
	if err != nil {
		return fmt.Errorf("cannot build select statement for raw flows consumer view: %w", err)
	}
	with := []string{}
	// c_DstAsPath
	if column, ok := c.d.Schema.LookupColumnByKey(schema.ColumnDstASPath); ok && !column.Disabled {
		with = append(with, "arrayCompact(DstASPath) AS c_DstASPath")
	}
	// c_SrcNetworks and c_DstNetworks
	lookups := dictionaryNetworksLookupRegex.FindAllStringSubmatch(selectQuery, -1)
	if len(lookups) > 0 {
		// Build the with clause
		srcColumns := []string{}
		dstColumns := []string{}
		for _, lookup := range lookups {
			if lookup[1] == "Src" {
				srcColumns = append(srcColumns, lookup[2])
			} else if lookup[1] == "Dst" {
				dstColumns = append(dstColumns, lookup[2])
			}
		}
		for _, columns := range []struct {
			direction string
			names     []string
		}{
			{direction: "Src", names: srcColumns},
			{direction: "Dst", names: dstColumns},
		} {
			if len(columns.names) > 0 {
				names := []string{}
				for _, column := range columns.names {
					names = append(names, fmt.Sprintf("'%s'", column))
				}
				with = append(with,
					fmt.Sprintf("dictGet('%s', (%s), %sAddr) AS c_%sNetworks",
						schema.DictionaryNetworks,
						strings.Join(names, ", "),
						columns.direction,
						columns.direction,
					))
			}
		}
		// Replace in query to use the index
		srcIdx := 0
		dstIdx := 0
		selectQuery = dictionaryNetworksLookupRegex.ReplaceAllStringFunc(selectQuery, func(match string) string {
			if strings.Contains(match, "Src") {
				srcIdx++
				return fmt.Sprintf("c_SrcNetworks.%d", srcIdx)
			} else if strings.Contains(match, "Dst") {
				dstIdx++
				return fmt.Sprintf("c_DstNetworks.%d", dstIdx)
			}
			return match
		})
	}
	if len(with) > 0 {
		selectQuery = fmt.Sprintf("WITH %s %s", strings.Join(with, ", "), selectQuery)
	}

	// Check the existing one
	if ok, err := c.tableAlreadyExists(ctx, viewName, "as_select", selectQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msg("raw flows consumer view already exists, skip migration")
		return errSkipStep
	}

	// Drop and create
	c.r.Info().Msg("create raw flows consumer view")
	if err := c.d.ClickHouse.ExecOnCluster(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s SYNC`, viewName)); err != nil {
		return fmt.Errorf("cannot drop table %s: %w", viewName, err)
	}
	if err := c.d.ClickHouse.ExecOnCluster(ctx,
		fmt.Sprintf("CREATE MATERIALIZED VIEW %s TO %s AS %s",
			viewName, c.distributedTable("flows"), selectQuery)); err != nil {
		return fmt.Errorf("cannot create raw flows consumer view: %w", err)
	}

	return nil
}

func (c *Component) createRawFlowsErrors(ctx context.Context) error {
	name := c.localTable("flows_raw_errors")
	createQuery, err := stemplate(`CREATE TABLE {{ .Database }}.{{ .Table }}
(`+"`timestamp`"+` DateTime,
 `+"`topic`"+` LowCardinality(String),
 `+"`partition`"+` UInt64,
 `+"`offset`"+` UInt64,
 `+"`raw`"+` String,
 `+"`error`"+` String)
ENGINE = {{ .Engine }}
PARTITION BY toYYYYMMDDhhmmss(toStartOfHour(timestamp))
ORDER BY (timestamp, topic, partition, offset)
TTL timestamp + toIntervalDay(1)
`, gin.H{
		"Table":    name,
		"Database": c.config.Database,
		"Engine":   c.mergeTreeEngine(name, ""),
	})
	if err != nil {
		return fmt.Errorf("cannot build query to create flow error table: %w", err)
	}
	if ok, err := c.tableAlreadyExists(ctx, name, "create_table_query", createQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msgf("table %s already exists, skip migration", name)
		return errSkipStep
	}
	c.r.Info().Msgf("create table %s", name)
	createOrReplaceQuery := strings.Replace(createQuery, "CREATE ", "CREATE OR REPLACE ", 1)
	if err := c.d.ClickHouse.ExecOnCluster(ctx, createOrReplaceQuery); err != nil {
		return fmt.Errorf("cannot create table %s: %w", name, err)
	}
	return nil
}

func (c *Component) createRawFlowsErrorsConsumerView(ctx context.Context) error {
	source := fmt.Sprintf("flows_%s_raw", c.d.Schema.ProtobufMessageHash())
	viewName := "flows_raw_errors_consumer"

	// Build SELECT query
	selectQuery, err := stemplate(`
SELECT
 now() AS timestamp,
 _topic AS topic,
 _partition AS partition,
 _offset AS offset,
 _raw_message AS raw,
 _error AS error
FROM {{ .Database }}.{{ .Table }}
WHERE length(_error) > 0`, gin.H{
		"Database": c.config.Database,
		"Table":    source,
	})
	if err != nil {
		return fmt.Errorf("cannot build select statement for raw flows error: %w", err)
	}

	// Check the existing one
	if ok, err := c.tableAlreadyExists(ctx, viewName, "as_select", selectQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msg("raw flows errors view already exists, skip migration")
		return errSkipStep
	}

	// Drop and create
	c.r.Info().Msg("create raw flows errors view")
	if err := c.d.ClickHouse.ExecOnCluster(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s SYNC`, viewName)); err != nil {
		return fmt.Errorf("cannot drop table %s: %w", viewName, err)
	}
	if err := c.d.ClickHouse.ExecOnCluster(ctx,
		fmt.Sprintf(`CREATE MATERIALIZED VIEW %s TO %s AS %s`,
			viewName, c.distributedTable("flows_raw_errors"), selectQuery)); err != nil {
		return fmt.Errorf("cannot create raw flows errors view: %w", err)
	}

	return nil
}

func (c *Component) deleteOldRawFlowsErrorsView(ctx context.Context) error {
	tableName := fmt.Sprintf("flows_%s_raw", c.d.Schema.ProtobufMessageHash())
	viewName := fmt.Sprintf("%s_errors", tableName)

	// Check the existing one
	if ok, err := c.tableAlreadyExists(ctx, viewName, "name", viewName); err != nil {
		return err
	} else if !ok {
		c.r.Debug().Msg("old raw flows errors view does not exist, skip migration")
		return errSkipStep
	}

	// Drop
	c.r.Info().Msg("delete old raw flows errors view")
	if err := c.d.ClickHouse.ExecOnCluster(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s SYNC`, viewName)); err != nil {
		return fmt.Errorf("cannot drop table %s: %w", viewName, err)
	}
	return nil
}

func (c *Component) createOrUpdateFlowsTable(ctx context.Context, resolution ResolutionConfiguration) error {
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
		"allow_suspicious_low_cardinality_types": 1,
	}))
	var tableName string
	if resolution.Interval == 0 {
		tableName = "flows"
	} else {
		tableName = fmt.Sprintf("flows_%s", resolution.Interval)
	}
	tableName = c.localTable(tableName)
	partitionInterval := uint64((resolution.TTL / time.Duration(c.config.MaxPartitions)).Seconds())
	ttl := uint64(resolution.TTL.Seconds())

	// Create table if it does not exist
	if ok, err := c.tableAlreadyExists(ctx, tableName, "name", tableName); err != nil {
		return err
	} else if !ok {
		var createQuery string
		var err error
		if resolution.Interval == 0 {
			createQuery, err = stemplate(`
CREATE TABLE {{ .Table }} ({{ .Schema }})
ENGINE = {{ .Engine }}
PARTITION BY toYYYYMMDDhhmmss(toStartOfInterval(TimeReceived, INTERVAL {{ .PartitionInterval }} second))
ORDER BY (toStartOfFiveMinutes(TimeReceived), ExporterAddress, InIfName, OutIfName)
TTL TimeReceived + toIntervalSecond({{ .TTL }})
`, gin.H{
				"Table":             tableName,
				"Schema":            c.d.Schema.ClickHouseCreateTable(),
				"PartitionInterval": partitionInterval,
				"TTL":               ttl,
				"Engine":            c.mergeTreeEngine(tableName, ""),
			})
		} else {
			createQuery, err = stemplate(`
CREATE TABLE {{ .Table }} ({{ .Schema }})
ENGINE = {{ .Engine }}
PARTITION BY toYYYYMMDDhhmmss(toStartOfInterval(TimeReceived, INTERVAL {{ .PartitionInterval }} second))
PRIMARY KEY ({{ .PrimaryKey }})
ORDER BY ({{ .SortingKey }})
TTL TimeReceived + toIntervalSecond({{ .TTL }})
`, gin.H{
				"Table":             tableName,
				"Schema":            c.d.Schema.ClickHouseCreateTable(schema.ClickHouseSkipMainOnlyColumns),
				"PartitionInterval": partitionInterval,
				"PrimaryKey":        strings.Join(c.d.Schema.ClickHousePrimaryKeys(), ", "),
				"SortingKey":        strings.Join(c.d.Schema.ClickHouseSortingKeys(), ", "),
				"TTL":               ttl,
				"Engine":            c.mergeTreeEngine(tableName, "Summing", "(Bytes, Packets)"),
			})
		}
		if err != nil {
			return fmt.Errorf("cannot build create table statement for %s: %w", tableName, err)
		}
		if err := c.d.ClickHouse.ExecOnCluster(ctx, createQuery); err != nil {
			return fmt.Errorf("cannot create %s: %w", tableName, err)
		}
		return nil
	}

	// Get existing columns
	var existingColumns []struct {
		Name             string `ch:"name"`
		Type             string `ch:"type"`
		CompressionCodec string `ch:"compression_codec"`
		IsSortingKey     uint8  `ch:"is_in_sorting_key"`
		IsPrimaryKey     uint8  `ch:"is_in_primary_key"`
		DefaultKind      string `ch:"default_kind"`
	}
	if err := c.d.ClickHouse.Select(ctx, &existingColumns, `
SELECT name, type, compression_codec, is_in_sorting_key, is_in_primary_key, default_kind
FROM system.columns
WHERE database = $1
AND table = $2
ORDER BY position ASC
`, c.config.Database, tableName); err != nil {
		return fmt.Errorf("cannot query columns table: %w", err)
	}

	// Plan for modifications. We don't check everything: we assume the
	// modifications to be done are covered by the unit tests.
	modifications := []string{}
	previousColumn := ""
outer:
	for _, wantedColumn := range c.d.Schema.Columns() {
		if resolution.Interval > 0 && wantedColumn.ClickHouseMainOnly {
			continue
		}
		// Check if the column already exists
		for _, existingColumn := range existingColumns {
			if wantedColumn.Name == existingColumn.Name {
				modifyTypeOrCodec := false
				if wantedColumn.ClickHouseType != existingColumn.Type {
					modifyTypeOrCodec = true
					if slices.Contains(c.d.Schema.ClickHousePrimaryKeys(), wantedColumn.Name) {
						return fmt.Errorf("table %s, primary key column %s has a non-matching type: %s vs %s",
							tableName, wantedColumn.Name, existingColumn.Type, wantedColumn.ClickHouseType)
					}
				}
				if wantedColumn.ClickHouseCodec != "" {
					wantedCodec := fmt.Sprintf("CODEC(%s)", wantedColumn.ClickHouseCodec)
					if wantedCodec != existingColumn.CompressionCodec {
						modifyTypeOrCodec = true
					}
				}
				// change alias existence has changed. ALIAS expression changes are not yet checked here.
				if (wantedColumn.ClickHouseAlias != "") != (existingColumn.DefaultKind == "ALIAS") {
					// either the column was an alias and should be none, or the other way around. Either way, we need to recreate.
					c.r.Logger.Debug().Msg(fmt.Sprintf("column %s alias content has changed, recreating. New ALIAS: %s", existingColumn.Name, wantedColumn.ClickHouseAlias))
					err := c.d.ClickHouse.ExecOnCluster(ctx,
						fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tableName, existingColumn.Name))
					if err != nil {
						return fmt.Errorf("cannot drop %s from %s to cleanup aliasing: %w",
							existingColumn.Name, tableName, err)
					}
					// Schedule adding it back
					modifications = append(modifications,
						fmt.Sprintf("ADD COLUMN %s AFTER %s", wantedColumn.ClickHouseDefinition(), previousColumn))
				}

				if resolution.Interval > 0 && slices.Contains(c.d.Schema.ClickHousePrimaryKeys(), wantedColumn.Name) && existingColumn.IsPrimaryKey == 0 {
					return fmt.Errorf("table %s, column %s should be a primary key, cannot change that",
						tableName, wantedColumn.Name)
				}
				if resolution.Interval > 0 && !wantedColumn.ClickHouseNotSortingKey && existingColumn.IsSortingKey == 0 {
					// That's something we can fix, but we need to drop it before recreating it
					err := c.d.ClickHouse.ExecOnCluster(ctx,
						fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tableName, existingColumn.Name))
					if err != nil {
						return fmt.Errorf("cannot drop %s from %s to fix ordering: %w",
							existingColumn.Name, tableName, err)
					}
					// Schedule adding it back
					modifications = append(modifications,
						fmt.Sprintf("ADD COLUMN %s AFTER %s", wantedColumn.ClickHouseDefinition(), previousColumn))
				} else if modifyTypeOrCodec {
					modifications = append(modifications,
						fmt.Sprintf("MODIFY COLUMN %s", wantedColumn.ClickHouseDefinition()))
				}
				previousColumn = wantedColumn.Name
				continue outer
			}
		}
		// Add the missing column. Only if not primary.
		if resolution.Interval > 0 && slices.Contains(c.d.Schema.ClickHousePrimaryKeys(), wantedColumn.Name) {
			return fmt.Errorf("table %s, column %s is missing but it is a primary key",
				tableName, wantedColumn.Name)
		}
		c.r.Debug().Msgf("add missing column %s to %s", wantedColumn.Name, tableName)
		modifications = append(modifications,
			fmt.Sprintf("ADD COLUMN %s AFTER %s", wantedColumn.ClickHouseDefinition(), previousColumn))
		previousColumn = wantedColumn.Name
	}
	if len(modifications) > 0 {
		// Also update ORDER BY
		if resolution.Interval > 0 {
			modifications = append(modifications,
				fmt.Sprintf("MODIFY ORDER BY (%s)", strings.Join(c.d.Schema.ClickHouseSortingKeys(), ", ")))
		}
		c.r.Info().Msgf("apply %d modifications to %s", len(modifications), tableName)
		if resolution.Interval > 0 {
			// Drop the view
			viewName := fmt.Sprintf("%s_consumer", tableName)
			if err := c.d.ClickHouse.ExecOnCluster(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s SYNC`, viewName)); err != nil {
				return fmt.Errorf("cannot drop %s: %w", viewName, err)
			}
		}
		err := c.d.ClickHouse.ExecOnCluster(ctx, fmt.Sprintf("ALTER TABLE %s %s", tableName, strings.Join(modifications, ", ")))
		if err != nil {
			return fmt.Errorf("cannot update table %s: %w", tableName, err)
		}
	}

	// Check if we need to update the TTL
	ttlClause := fmt.Sprintf("TTL TimeReceived + toIntervalSecond(%d)", ttl)
	ttlClauseLike := fmt.Sprintf("CAST(engine_full LIKE '%% %s %%', 'String')", ttlClause)
	if ok, err := c.tableAlreadyExists(ctx, tableName, ttlClauseLike, "1"); err != nil {
		return err
	} else if !ok {
		c.r.Warn().
			Msgf("updating TTL of %s with interval %s, this can take a long time", tableName, resolution.Interval)
		if err := c.d.ClickHouse.ExecOnCluster(ctx, fmt.Sprintf("ALTER TABLE %s MODIFY %s", tableName, ttlClause)); err != nil {
			return fmt.Errorf("cannot modify TTL for table %s: %w", tableName, err)
		}
		return nil
	} else if len(modifications) > 0 {
		return nil
	}
	return errSkipStep
}

func (c *Component) createFlowsConsumerView(ctx context.Context, resolution ResolutionConfiguration) error {
	if resolution.Interval == 0 {
		// The consumer for the main table is created elsewhere.
		return errSkipStep
	}
	tableName := fmt.Sprintf("flows_%s", resolution.Interval)
	viewName := fmt.Sprintf("%s_consumer", tableName)

	// Build SELECT query
	selectQuery, err := stemplate(`
SELECT
 toStartOfInterval(TimeReceived, toIntervalSecond({{ .Seconds }})) AS TimeReceived,
 {{ .Columns }}
FROM {{ .Database }}.{{ .Table }}`, gin.H{
		"Database": c.config.Database,
		"Table":    c.localTable("flows"),
		"Seconds":  uint64(resolution.Interval.Seconds()),
		"Columns": strings.Join(c.d.Schema.ClickHouseSelectColumns(
			schema.ClickHouseSkipTimeReceived,
			schema.ClickHouseSkipMainOnlyColumns,
			schema.ClickHouseSkipAliasedColumns), ",\n "),
	})
	if err != nil {
		return fmt.Errorf("cannot build select statement for consumer %s: %w", viewName, err)
	}

	// Check the existing one
	if ok, err := c.tableAlreadyExists(ctx, viewName, "as_select", selectQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msgf("%s already exists, skip migration", viewName)
		return errSkipStep
	}

	// Drop and create
	c.r.Info().Msgf("create %s", viewName)
	if err := c.d.ClickHouse.ExecOnCluster(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s SYNC`, viewName)); err != nil {
		return fmt.Errorf("cannot drop table %s: %w", viewName, err)
	}
	if err := c.d.ClickHouse.ExecOnCluster(ctx,
		fmt.Sprintf(`CREATE MATERIALIZED VIEW %s TO %s AS %s`, viewName,
			c.localTable(tableName), selectQuery)); err != nil {
		return fmt.Errorf("cannot create %s: %w", viewName, err)
	}
	return nil
}

// createDistributedTable creates the distributed version of an existing table.
// If the table already exists and does not match the definition, it is
// replaced.
func (c *Component) createDistributedTable(ctx context.Context, source string) error {
	if c.config.Cluster == "" {
		return errSkipStep
	}
	// Get the schema of the source table
	var existingColumns []struct {
		Name              string `ch:"name"`
		Type              string `ch:"type"`
		CompressionCodec  string `ch:"compression_codec"`
		DefaultKind       string `ch:"default_kind"`
		DefaultExpression string `ch:"default_expression"`
	}
	if err := c.d.ClickHouse.Select(ctx, &existingColumns, `
SELECT name, type, compression_codec, default_kind, default_expression
FROM system.columns
WHERE database = $1 AND table = $2
ORDER BY position ASC
`, c.config.Database, c.localTable(source)); err != nil {
		return fmt.Errorf("cannot query columns table: %w", err)
	}
	cols := []string{}
	for _, column := range existingColumns {
		col := fmt.Sprintf("`%s` %s", column.Name, column.Type)
		if column.CompressionCodec != "" {
			col = fmt.Sprintf("%s %s", col, column.CompressionCodec)
		}
		if column.DefaultKind != "" {
			col = fmt.Sprintf("%s %s %s", col, column.DefaultKind, column.DefaultExpression)
		}
		cols = append(cols, col)
	}

	// Build the CREATE TABLE
	createQuery, err := stemplate(
		`CREATE TABLE {{ .Database }}.{{ .Target }}
({{ .Schema }})
ENGINE = Distributed('{{ .Cluster }}', '{{ .Database}}', '{{ .Source }}', rand())`,
		gin.H{
			"Cluster":  c.config.Cluster,
			"Database": c.config.Database,
			"Source":   c.localTable(source),
			"Target":   c.distributedTable(source),
			"Schema":   strings.Join(cols, ", "),
		})
	if err != nil {
		return fmt.Errorf("cannot build query to create exporters view: %w", err)
	}

	// Check if the table already exists
	if ok, err := c.tableAlreadyExists(ctx, c.distributedTable(source), "create_table_query", createQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msgf("%s already exists, skip migration", c.distributedTable(source))
		return errSkipStep
	}

	// Recreate the table
	c.r.Info().Msgf("create %s", c.distributedTable(source))
	createOrReplaceQuery := strings.Replace(createQuery, "CREATE ", "CREATE OR REPLACE ", 1)
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
		"allow_suspicious_low_cardinality_types": 1,
	}))
	if err := c.d.ClickHouse.ExecOnCluster(ctx, createOrReplaceQuery); err != nil {
		return fmt.Errorf("cannot create %s: %w", c.distributedTable(source), err)
	}
	return nil
}
