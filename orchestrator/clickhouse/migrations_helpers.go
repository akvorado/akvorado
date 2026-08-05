// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	sb "akvorado/common/sqlbuilder"
)

var errSkipStep = errors.New("migration: skip this step")

// defaultTableSettingsKeys lists the default table setting keys in their fixed output order.
var defaultTableSettingsKeys = []string{"index_granularity", "ttl_only_drop_parts"}

// defaultTableSettings are the default ClickHouse table settings applied to flow tables.
var defaultTableSettings = TableSettings{
	"index_granularity":   8192,
	"ttl_only_drop_parts": 1,
}

// tableSetting is one ClickHouse table setting with its value.
type tableSetting struct {
	name  string
	value any
}

// expr returns the value of the setting as a SQL expression.
func (s tableSetting) expr() sb.Expr {
	switch value := s.value.(type) {
	case int:
		return sb.Int(int64(value))
	case string:
		return sb.String(value)
	}
	return sb.Expr{}
}

// String renders the setting the way ClickHouse writes it back in engine_full.
func (s tableSetting) String() string {
	return fmt.Sprintf("%s = %s", s.name, s.expr())
}

// tableSettings merges extra settings with the default table settings and
// returns them in a stable order. Default settings come first
// (index_granularity, ttl_only_drop_parts), followed by extra settings sorted
// alphabetically.
func tableSettings(extra TableSettings) []tableSetting {
	merged := TableSettings{}
	maps.Copy(merged, defaultTableSettings)
	maps.Copy(merged, extra)

	extraKeys := make([]string, 0, len(merged))
	for k := range merged {
		if !slices.Contains(defaultTableSettingsKeys, k) {
			extraKeys = append(extraKeys, k)
		}
	}
	slices.Sort(extraKeys)

	settings := []tableSetting{}
	for _, k := range slices.Concat(defaultTableSettingsKeys, extraKeys) {
		switch merged[k].(type) {
		case int, string:
			settings = append(settings, tableSetting{name: k, value: merged[k]})
		}
	}
	return settings
}

// renderTableSettings renders the settings the way ClickHouse writes them back,
// so they can be looked for in engine_full.
func renderTableSettings(extra TableSettings) string {
	settings := tableSettings(extra)
	parts := make([]string, len(settings))
	for i, setting := range settings {
		parts[i] = setting.String()
	}
	return strings.Join(parts, ", ")
}

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

// table names a table of the database akvorado uses.
func (c *Component) table(name string) sb.TableName {
	return sb.Table(name).In(c.d.ClickHouse.DatabaseName())
}

// statement is a SQL statement we can compare with what ClickHouse kept about
// an existing table.
type statement interface {
	fmt.Stringer
	Matches(sql string) bool
}

// settingsRegexp matches the settings ClickHouse appends to a table
// definition. They are checked on their own, see createOrUpdateFlowsTable.
var settingsRegexp = regexp.MustCompile(` SETTINGS .*$`)

// tableColumn fetches one column of system.tables for the provided table. The
// column can be any expression. An empty string is returned when the table does
// not exist.
func (c *Component) tableColumn(ctx context.Context, table, column string) (string, error) {
	row := c.d.ClickHouse.QueryRow(ctx,
		fmt.Sprintf("SELECT %s FROM system.tables WHERE name = $1 AND database = $2", column),
		table, c.d.ClickHouse.DatabaseName())
	var existing string
	if err := row.Scan(&existing); err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("cannot check if table %s already exists: %w", table, err)
	}
	return existing, nil
}

// tableAlreadyExists compares the table in the database with the statement we
// would use to create it. `column` is either "create_table_query" or
// "as_select".
func (c *Component) tableAlreadyExists(ctx context.Context, table, column string, target statement) (bool, error) {
	existing, err := c.tableColumn(ctx, table, column)
	if err != nil {
		return false, err
	}
	// ClickHouse adds the database in front of the dictionaries, we do not.
	for _, function := range []string{"dictGetOrDefault", "dictGet"} {
		existing = strings.ReplaceAll(existing,
			fmt.Sprintf("%s('%s.", function, c.d.ClickHouse.DatabaseName()),
			fmt.Sprintf("%s('", function))
	}
	existing = settingsRegexp.ReplaceAllString(existing, "")

	if target.Matches(existing) {
		return true, nil
	}
	c.r.Debug().
		Str("target", target.String()).Str("existing", existing).
		Msgf("table %s state difference detected", table)
	return false, nil
}

// mergeTreeEngine returns a MergeTree engine definition, either plain or using
// Replicated if we are on a cluster.
func (c *Component) mergeTreeEngine(table, variant string, args ...sb.Expr) sb.Engine {
	if c.d.ClickHouse.ClusterName() != "" {
		zkPath := fmt.Sprintf("/clickhouse/tables/shard-{shard}/%s", table)
		if helpers.Testing() {
			zkPath = fmt.Sprintf("/clickhouse/tables/shard-{shard}/%s/%s",
				c.d.ClickHouse.DatabaseName(), table)
		}
		return sb.NewEngine(fmt.Sprintf("Replicated%sMergeTree", variant),
			slices.Concat([]sb.Expr{
				sb.String(zkPath),
				sb.String("replica-{replica}"),
			}, args)...)
	}
	return sb.NewEngine(fmt.Sprintf("%sMergeTree", variant), args...)
}

// distributedTable turns a table name to the matching distributed table if we
// are in a cluster.
func (c *Component) distributedTable(table string) string {
	return table
}

// localTable turns a table name to the matching local distributed table if we
// are in a cluster.
func (c *Component) localTable(table string) string {
	if c.d.ClickHouse.ClusterName() != "" && c.shards > 1 {
		return fmt.Sprintf("%s_local", table)
	}
	return table
}

// createDictionary creates the provided dictionary.
func (c *Component) createDictionary(ctx context.Context, name, layout string, attributes []sb.DictionaryAttribute, keys []sb.Expr) error {
	url := fmt.Sprintf("%s/api/v0/orchestrator/clickhouse/%s.csv", c.config.OrchestratorURL, name)
	source := []sb.SourceParam{
		sb.Param("URL", sb.String(url)),
		sb.Param("FORMAT", sb.String("CSVWithNames")),
	}
	if c.config.OrchestratorBasicAuth != nil {
		source = append(source, sb.ParamGroup("CREDENTIALS",
			sb.Param("user", sb.String(c.config.OrchestratorBasicAuth.Username)),
			sb.Param("password", sb.String(c.config.OrchestratorBasicAuth.Password))))
	}
	createQuery := sb.CreateDictionary(c.table(name)).
		Attributes(attributes...).
		PrimaryKey(keys...).
		Source("HTTP", source...).
		Lifetime(0, 3600).
		Layout(strings.ToUpper(layout)).
		Setting("format_csv_allow_single_quotes", sb.Uint(0))

	// Check if dictionary exists and create it if not
	if ok, err := c.tableAlreadyExists(ctx, name, "create_table_query", createQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msgf("dictionary %s already exists, skip migration", name)
		return errSkipStep
	}
	c.r.Info().Msgf("create dictionary %s", name)
	if err := c.d.ClickHouse.ExecOnCluster(ctx, createQuery.OrReplace()); err != nil {
		return fmt.Errorf("cannot create dictionary %s: %w", name, err)
	}
	return nil
}

// createExportersTable creates the exporters table. This table is always local.
func (c *Component) createExportersTable(ctx context.Context) error {
	// Select the columns we need. Codecs and aliases are not carried over.
	columns := []sb.ColumnDef{}
	for _, column := range c.d.Schema.Columns() {
		if column.Key == schema.ColumnTimeReceived || strings.HasPrefix(column.Name, "Exporter") {
			columns = append(columns, sb.NewColumnDef(column.Name, column.ClickHouseType))
		}
		if strings.HasPrefix(column.Name, "InIf") {
			columns = append(columns,
				sb.NewColumnDef(column.Name[2:], column.ClickHouseType))
		}
	}

	// Build CREATE TABLE
	name := "exporters"
	createQuery := sb.CreateTable(c.table(name)).
		Columns(columns...).
		Engine(c.mergeTreeEngine(name, "Replacing", sb.Column("TimeReceived"))).
		OrderBy(sb.Columns("ExporterAddress", "IfName")...).
		TTL(sb.Op(sb.Column("TimeReceived"), "+",
			sb.Function("toIntervalDay", sb.Uint(1))))

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
	if err := c.d.ClickHouse.ExecOnCluster(ctx, createQuery.OrReplace()); err != nil {
		return fmt.Errorf("cannot create exporters table: %w", err)
	}

	return nil
}

// createExportersConsumerView creates the exporters view.
func (c *Component) createExportersConsumerView(ctx context.Context) error {
	// Select the columns we need. The In/Out pair of an interface column
	// becomes two rows, picked by the ARRAY JOIN below.
	items := []sb.Expr{}
	for _, column := range c.d.Schema.Columns() {
		if column.Key == schema.ColumnTimeReceived || strings.HasPrefix(column.Name, "Exporter") {
			items = append(items, sb.Column(column.Name))
		}
		if strings.HasPrefix(column.Name, "InIf") {
			suffix := column.Name[4:]
			items = append(items, sb.Alias(
				sb.Index(sb.Array(
					sb.Column(fmt.Sprintf("InIf%s", suffix)),
					sb.Column(fmt.Sprintf("OutIf%s", suffix))),
					sb.Column("num")),
				fmt.Sprintf("If%s", suffix)))
		}
	}

	// Build SELECT query
	name := "exporters_consumer"
	selectQuery := sb.Select(items...).
		From(c.table(c.distributedTable("flows"))).
		ArrayJoin(sb.Alias(
			sb.Function("arrayEnumerate", sb.Array(sb.Uint(1), sb.Uint(2))), "num"))

	// Check if the table already exists with these columns and with a TTL.
	if ok, err := c.tableAlreadyExists(ctx, name, "as_select", selectQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msg("exporters view already exists, skip migration")
		return errSkipStep
	}

	// Drop existing table and recreate
	c.r.Info().Msg("create exporters view")
	if err := c.d.ClickHouse.ExecOnCluster(ctx, sb.DropTable(sb.Table(name))); err != nil {
		return fmt.Errorf("cannot drop existing exporters view: %w", err)
	}
	if err := c.d.ClickHouse.ExecOnCluster(ctx, sb.CreateMaterializedView(
		sb.Table(name), sb.Table("exporters"), selectQuery)); err != nil {
		return fmt.Errorf("cannot create exporters view: %w", err)
	}

	return nil
}

// createRawFlowsTable creates the raw flow table
func (c *Component) createRawFlowsTable(ctx context.Context) error {
	hash := c.d.Schema.ClickHouseHash()
	tableName := fmt.Sprintf("flows_%s_raw", hash)

	// Build CREATE query
	columns, err := sb.ParseColumnDefs(c.d.Schema.ClickHouseCreateTable(
		schema.ClickHouseSkipGeneratedColumns,
		schema.ClickHouseSkipAliasedColumns))
	if err != nil {
		return fmt.Errorf("cannot build query to create raw flows table: %w", err)
	}
	createQuery := sb.CreateTable(c.table(tableName)).
		Columns(columns...).
		Engine(sb.NewEngine("Null"))

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
		tableName,
	} {
		if err := c.d.ClickHouse.ExecOnCluster(ctx, sb.DropTable(sb.Table(table))); err != nil {
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

func (c *Component) createRawFlowsConsumerView(ctx context.Context) error {
	tableName := fmt.Sprintf("flows_%s_raw", c.d.Schema.ClickHouseHash())
	viewName := fmt.Sprintf("%s_consumer", tableName)

	// Build SELECT query
	items, err := sb.ParseExprs(c.d.Schema.ClickHouseSelectColumns(
		schema.ClickHouseSubstituteGenerates,
		schema.ClickHouseSkipAliasedColumns))
	if err != nil {
		return fmt.Errorf("cannot build select statement for raw flows consumer view: %w", err)
	}
	selectQuery := sb.Select(items...).From(c.table(tableName))
	// c_DstAsPath
	if column, ok := c.d.Schema.LookupColumnByKey(schema.ColumnDstASPath); ok && !column.Disabled {
		selectQuery.WithAlias(
			sb.Function("arrayCompact", sb.Column("DstASPath")), "c_DstASPath")
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
	if err := c.d.ClickHouse.ExecOnCluster(ctx, sb.DropTable(sb.Table(viewName))); err != nil {
		return fmt.Errorf("cannot drop table %s: %w", viewName, err)
	}
	if err := c.d.ClickHouse.ExecOnCluster(ctx, sb.CreateMaterializedView(
		sb.Table(viewName), sb.Table(c.distributedTable("flows")),
		selectQuery)); err != nil {
		return fmt.Errorf("cannot create raw flows consumer view: %w", err)
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
	ttlExpr := sb.Op(sb.Column("TimeReceived"), "+",
		sb.Function("toIntervalSecond", sb.Uint(ttl)))
	settings := tableSettings(resolution.TableSettings)

	// Create table if it does not exist
	if existing, err := c.tableColumn(ctx, tableName, "name"); err != nil {
		return err
	} else if existing == "" {
		options := []schema.ClickHouseTableOption{}
		if resolution.Interval > 0 {
			options = append(options, schema.ClickHouseSkipMainOnlyColumns)
		}
		columns, err := sb.ParseColumnDefs(c.d.Schema.ClickHouseCreateTable(options...))
		if err != nil {
			return fmt.Errorf("cannot build create table statement for %s: %w", tableName, err)
		}
		createQuery := sb.CreateTable(sb.Table(tableName)).
			Columns(columns...).
			PartitionBy(sb.Function("toYYYYMMDDhhmmss",
				sb.Function("toStartOfInterval", sb.Column("TimeReceived"),
					sb.Interval(sb.Uint(partitionInterval), "second")))).
			TTL(ttlExpr)
		if resolution.Interval == 0 {
			fiveMinutes := sb.Function("toStartOfFiveMinutes", sb.Column("TimeReceived"))
			createQuery.
				Engine(c.mergeTreeEngine(tableName, "")).
				PrimaryKey(fiveMinutes).
				OrderBy(slices.Concat([]sb.Expr{fiveMinutes},
					sb.Columns("ExporterAddress", "InIfName", "OutIfName"))...)
		} else {
			createQuery.
				Engine(c.mergeTreeEngine(tableName, "Summing",
					sb.Tuple(sb.Columns("Bytes", "Packets")...))).
				PrimaryKey(sb.Columns(c.d.Schema.ClickHousePrimaryKeys()...)...).
				OrderBy(sb.Columns(c.d.Schema.ClickHouseSortingKeys()...)...)
		}
		for _, setting := range settings {
			createQuery.Setting(setting.name, setting.expr())
		}
		if err := c.d.ClickHouse.ExecOnCluster(ctx, createQuery); err != nil {
			return fmt.Errorf("cannot create %s: %w", tableName, err)
		}
		if _, err := c.applySkipIndexes(ctx, tableName, resolution.Interval == 0); err != nil {
			return err
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
`, c.d.ClickHouse.DatabaseName(), tableName); err != nil {
		return fmt.Errorf("cannot query columns table: %w", err)
	}

	// Plan for modifications. We don't check everything: we assume the
	// modifications to be done are covered by the unit tests.
	alterQuery := sb.AlterTable(sb.Table(tableName))
	previousColumn := ""
outer:
	for _, wantedColumn := range c.d.Schema.Columns() {
		if resolution.Interval > 0 && wantedColumn.ClickHouseMainOnly {
			continue
		}
		wantedDefinition, err := sb.ParseColumnDef(wantedColumn.ClickHouseDefinition())
		if err != nil {
			return fmt.Errorf("cannot parse the definition of column %s: %w", wantedColumn.Name, err)
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
					c.r.Debug().Msg(fmt.Sprintf("column %s alias content has changed, recreating. New ALIAS: %s", existingColumn.Name, wantedColumn.ClickHouseAlias))
					err := c.d.ClickHouse.ExecOnCluster(ctx,
						sb.AlterTable(sb.Table(tableName)).DropColumn(existingColumn.Name))
					if err != nil {
						return fmt.Errorf("cannot drop %s from %s to cleanup aliasing: %w",
							existingColumn.Name, tableName, err)
					}
					// Schedule adding it back
					alterQuery.AddColumn(wantedDefinition, previousColumn)
				}

				if resolution.Interval > 0 && slices.Contains(c.d.Schema.ClickHousePrimaryKeys(), wantedColumn.Name) && existingColumn.IsPrimaryKey == 0 {
					return fmt.Errorf("table %s, column %s should be a primary key, cannot change that",
						tableName, wantedColumn.Name)
				}
				if resolution.Interval > 0 && !wantedColumn.ClickHouseNotSortingKey && existingColumn.IsSortingKey == 0 {
					// That's something we can fix, but we need to drop it before recreating it
					err := c.d.ClickHouse.ExecOnCluster(ctx,
						sb.AlterTable(sb.Table(tableName)).DropColumn(existingColumn.Name))
					if err != nil {
						return fmt.Errorf("cannot drop %s from %s to fix ordering: %w",
							existingColumn.Name, tableName, err)
					}
					// Schedule adding it back
					alterQuery.AddColumn(wantedDefinition, previousColumn)
				} else if modifyTypeOrCodec {
					alterQuery.ModifyColumn(wantedDefinition)
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
		alterQuery.AddColumn(wantedDefinition, previousColumn)
		previousColumn = wantedColumn.Name
	}
	modified := false
	if alterQuery.Len() > 0 {
		// Also update ORDER BY
		if resolution.Interval > 0 {
			alterQuery.ModifyOrderBy(sb.Columns(c.d.Schema.ClickHouseSortingKeys()...)...)
		}
		c.r.Info().Msgf("apply %d modifications to %s", alterQuery.Len(), tableName)
		if resolution.Interval > 0 {
			// Drop the view
			viewName := fmt.Sprintf("%s_consumer", tableName)
			if err := c.d.ClickHouse.ExecOnCluster(ctx, sb.DropTable(sb.Table(viewName))); err != nil {
				return fmt.Errorf("cannot drop %s: %w", viewName, err)
			}
		}
		if err := c.d.ClickHouse.ExecOnCluster(ctx, alterQuery); err != nil {
			return fmt.Errorf("cannot update table %s: %w", tableName, err)
		}
		modified = true
	}

	// Check if we need to update the settings. They are the last part of the
	// engine, so the pattern is not open on the right.
	if ok, err := c.engineFullMatches(ctx, tableName,
		fmt.Sprintf("%% SETTINGS %s", renderTableSettings(resolution.TableSettings))); err != nil {
		return err
	} else if !ok {
		c.r.Info().Msgf("updating settings of %s to %s", tableName, resolution.Interval)
		alterSettings := sb.AlterTable(sb.Table(tableName))
		for _, setting := range settings {
			alterSettings.ModifySetting(setting.name, setting.expr())
		}
		if err := c.d.ClickHouse.ExecOnCluster(ctx, alterSettings); err != nil {
			return fmt.Errorf("cannot modify settings for table %s: %w", tableName, err)
		}
		modified = true
	}

	// Check if we need to update the TTL
	if ok, err := c.engineFullMatches(ctx, tableName,
		fmt.Sprintf("%% TTL %s %%", ttlExpr)); err != nil {
		return err
	} else if !ok {
		c.r.Info().Msgf("updating TTL of %s with interval %s", tableName, resolution.Interval)
		err := c.d.ClickHouse.ExecOnCluster(
			clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
				"materialize_ttl_after_modify": 0,
			})),
			sb.AlterTable(sb.Table(tableName)).ModifyTTL(ttlExpr))
		if err != nil {
			return fmt.Errorf("cannot modify TTL for table %s: %w", tableName, err)
		}
		modified = true
	}

	changed, err := c.applySkipIndexes(ctx, tableName, resolution.Interval == 0)
	if err != nil {
		return err
	}
	if changed {
		modified = true
	}

	if modified {
		return nil
	}
	return errSkipStep
}

// engineFullMatches tells if the engine of the table matches the provided LIKE
// pattern. This is how the settings and the TTL of an existing table are
// checked: they are not part of the create_table_query ClickHouse keeps.
func (c *Component) engineFullMatches(ctx context.Context, tableName, pattern string) (bool, error) {
	value, err := c.tableColumn(ctx, tableName,
		fmt.Sprintf("CAST(engine_full LIKE %s, 'String')", sb.String(pattern)))
	if err != nil {
		return false, err
	}
	return value == "1", nil
}

// applySkipIndexes reconciles the skip indexes on tableName with the configured
// schema indexes. It returns true if any change was made.
func (c *Component) applySkipIndexes(ctx context.Context, tableName string, isMainTable bool) (bool, error) {
	skipIndexes := c.d.Schema.GetSkipIndexes()
	toDrop := sb.AlterTable(sb.Table(tableName))
	toAdd := sb.AlterTable(sb.Table(tableName))

	// Collect existing skip indexes.
	existingIndexes := map[string]string{}
	if err := func() error {
		rows, err := c.d.ClickHouse.Query(ctx,
			`SELECT name, type_full FROM system.data_skipping_indices WHERE database = $1 AND table = $2 AND startsWith(name, 'idx_')`,
			c.d.ClickHouse.DatabaseName(), tableName)
		if err != nil {
			return fmt.Errorf("cannot list skip indices for %s: %w", tableName, err)
		}
		defer rows.Close()
		for rows.Next() {
			var name, typeFull string
			if err := rows.Scan(&name, &typeFull); err != nil {
				return fmt.Errorf("cannot scan skip index: %w", err)
			}
			existingIndexes[name] = typeFull
		}
		return rows.Err()
	}(); err != nil {
		return false, err
	}

	// Determine which indexes are wanted and what changes are needed.
	wantedIndexNames := map[string]struct{}{}
	for _, colKey := range slices.Sorted(maps.Keys(skipIndexes)) {
		idxType := skipIndexes[colKey]
		col, ok := c.d.Schema.LookupColumnByKey(colKey)
		if !ok || col.Disabled {
			continue
		}
		// ClickHouseMainOnly columns are absent from aggregated tables.
		if col.ClickHouseMainOnly && !isMainTable {
			continue
		}
		chType, err := idxType.ClickHouseType()
		if err != nil {
			return false, fmt.Errorf("schema index for %s: %w", col.Name, err)
		}
		indexType, err := sb.ParseExpr(chType)
		if err != nil {
			return false, fmt.Errorf("schema index for %s: %w", col.Name, err)
		}
		idxName := fmt.Sprintf("idx_%s", strings.ToLower(col.Name))
		wantedIndexNames[idxName] = struct{}{}

		existingType := existingIndexes[idxName]
		if existingType != "" && existingType != chType {
			toDrop.DropIndex(idxName)
		}
		if existingType == "" || existingType != chType {
			toAdd.AddIndex(idxName, sb.Column(col.Name), indexType, 4)
		}
	}

	// Drop stale skip indexes that are no longer wanted.
	for name := range existingIndexes {
		if _, wanted := wantedIndexNames[name]; !wanted {
			toDrop.DropIndex(name)
		}
	}

	if toDrop.Len() == 0 && toAdd.Len() == 0 {
		return false, nil
	}

	// Batch drops before adds (drop must precede re-add when type changes).
	if toDrop.Len() > 0 {
		c.r.Info().Msgf("removing %d skip index(es) from %s", toDrop.Len(), tableName)
		if err := c.d.ClickHouse.ExecOnCluster(ctx, toDrop); err != nil {
			return false, fmt.Errorf("cannot drop skip indexes on %s: %w", tableName, err)
		}
	}
	if toAdd.Len() > 0 {
		c.r.Info().Msgf("adding %d skip index(es) to %s", toAdd.Len(), tableName)
		if err := c.d.ClickHouse.ExecOnCluster(ctx, toAdd); err != nil {
			return false, fmt.Errorf("cannot add skip indexes on %s: %w", tableName, err)
		}
	}
	return true, nil
}

func (c *Component) createFlowsConsumerView(ctx context.Context, resolution ResolutionConfiguration) error {
	if resolution.Interval == 0 {
		// The consumer for the main table is created elsewhere.
		return errSkipStep
	}
	tableName := fmt.Sprintf("flows_%s", resolution.Interval)
	viewName := fmt.Sprintf("%s_consumer", tableName)

	// Build SELECT query
	columns, err := sb.ParseExprs(c.d.Schema.ClickHouseSelectColumns(
		schema.ClickHouseSkipTimeReceived,
		schema.ClickHouseSkipMainOnlyColumns,
		schema.ClickHouseSkipAliasedColumns))
	if err != nil {
		return fmt.Errorf("cannot build select statement for consumer %s: %w", viewName, err)
	}
	items := slices.Concat([]sb.Expr{
		sb.Alias(sb.Function("toStartOfInterval", sb.Column("TimeReceived"),
			sb.Function("toIntervalSecond", sb.Uint(uint64(resolution.Interval.Seconds())))),
			"TimeReceived"),
	}, columns)
	selectQuery := sb.Select(items...).From(c.table(c.localTable("flows")))

	// Check the existing one
	if ok, err := c.tableAlreadyExists(ctx, viewName, "as_select", selectQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msgf("%s already exists, skip migration", viewName)
		return errSkipStep
	}

	// Drop and create
	c.r.Info().Msgf("create %s", viewName)
	if err := c.d.ClickHouse.ExecOnCluster(ctx, sb.DropTable(sb.Table(viewName))); err != nil {
		return fmt.Errorf("cannot drop table %s: %w", viewName, err)
	}
	if err := c.d.ClickHouse.ExecOnCluster(ctx, sb.CreateMaterializedView(
		sb.Table(viewName), sb.Table(c.localTable(tableName)),
		selectQuery)); err != nil {
		return fmt.Errorf("cannot create %s: %w", viewName, err)
	}
	return nil
}

// createDistributedTable creates the distributed version of an existing table.
// If the table already exists and does not match the definition, it is
// replaced.
func (c *Component) createDistributedTable(ctx context.Context, source string) error {
	if c.localTable(source) == c.distributedTable(source) {
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
`, c.d.ClickHouse.DatabaseName(), c.localTable(source)); err != nil {
		return fmt.Errorf("cannot query columns table: %w", err)
	}
	// The columns are copied from the local table as ClickHouse writes them.
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
	columns, err := sb.ParseColumnDefs(strings.Join(cols, ", "))
	if err != nil {
		return fmt.Errorf("cannot parse the schema of %s: %w", c.localTable(source), err)
	}

	// Build the CREATE TABLE
	createQuery := sb.CreateTable(c.table(c.distributedTable(source))).
		Columns(columns...).
		Engine(sb.NewEngine("Distributed",
			sb.String(c.d.ClickHouse.ClusterName()),
			sb.String(c.d.ClickHouse.DatabaseName()),
			sb.String(c.localTable(source)),
			sb.Function("rand")))

	// Check if the table already exists
	if ok, err := c.tableAlreadyExists(ctx, c.distributedTable(source), "create_table_query", createQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msgf("%s already exists, skip migration", c.distributedTable(source))
		return errSkipStep
	}

	// Recreate the table
	c.r.Info().Msgf("create %s", c.distributedTable(source))
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
		"allow_suspicious_low_cardinality_types": 1,
	}))
	if err := c.d.ClickHouse.ExecOnCluster(ctx, createQuery.OrReplace()); err != nil {
		return fmt.Errorf("cannot create %s: %w", c.distributedTable(source), err)
	}
	return nil
}
