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

	"github.com/gin-gonic/gin"
	"golang.org/x/exp/slices"

	"akvorado/common/schema"
)

var errSkipStep = errors.New("migration: skip this step")

// wrapMigrations can be used to wrap migration functions. It will keep the
// metrics up-to-date as long as the migration function returns `errSkipStep`
// when a step is skipped.
func (c *Component) wrapMigrations(fns ...func() error) error {
	for _, fn := range fns {
		if err := fn(); err == nil {
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
	tpl, err := template.New("tpl").Parse(t)
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
		return false, fmt.Errorf("cannot check if table %s already exists", table)
	}
	existing = strings.ReplaceAll(existing,
		fmt.Sprintf(`dictGetOrDefault('%s.`, c.config.Database),
		"dictGetOrDefault('")

	// Compare!
	if existing == target {
		return true, nil
	}
	c.r.Debug().
		Str("target", target).Str("existing", existing).
		Msgf("table %s is not in the expected state", table)
	return false, nil
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
	if err := c.d.ClickHouse.Exec(ctx, createOrReplaceQuery); err != nil {
		return fmt.Errorf("cannot create dictionary %s: %w", name, err)
	}
	return nil
}

// createExportersView creates the exporters table/view.
func (c *Component) createExportersView(ctx context.Context) error {
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
		`SELECT DISTINCT {{ .Columns }} FROM {{ .Database }}.flows ARRAY JOIN arrayEnumerate([1, 2]) AS num`,
		gin.H{
			"Database": c.config.Database,
			"Columns":  strings.Join(cols, ", "),
		})
	if err != nil {
		return fmt.Errorf("cannot build query to create exporters view: %w", err)
	}

	// Check if the table already exists with these columns and with a TTL.
	if ok, err := c.tableAlreadyExists(ctx,
		"exporters",
		"IF(position(create_table_query, 'TTL TimeReceived ') > 0, as_select, 'NOTTL')",
		selectQuery); err != nil {
		return err
	} else if ok {
		c.r.Info().Msg("exporters view already exists, skip migration")
		return errSkipStep
	}

	// Drop existing table and recreate
	c.r.Info().Msg("create exporters view")
	if err := c.d.ClickHouse.Exec(ctx, `DROP TABLE IF EXISTS exporters SYNC`); err != nil {
		return fmt.Errorf("cannot drop existing exporters view: %w", err)
	}
	if err := c.d.ClickHouse.Exec(ctx, fmt.Sprintf(`
CREATE MATERIALIZED VIEW exporters
ENGINE = ReplacingMergeTree(TimeReceived)
ORDER BY (ExporterAddress, IfName)
TTL TimeReceived + INTERVAL 1 DAY
AS %s
`, selectQuery)); err != nil {
		return fmt.Errorf("cannot create exporters view: %w", err)
	}

	return nil
}

// createRawFlowsTable creates the raw flow table
func (c *Component) createRawFlowsTable(ctx context.Context) error {
	hash := c.d.Schema.ProtobufMessageHash()
	tableName := fmt.Sprintf("flows_%s_raw", hash)
	kafkaEngine := fmt.Sprintf("Kafka SETTINGS %s", strings.Join([]string{
		fmt.Sprintf(`kafka_broker_list = '%s'`,
			strings.Join(c.config.Kafka.Brokers, ",")),
		fmt.Sprintf(`kafka_topic_list = '%s-%s'`,
			c.config.Kafka.Topic, hash),
		`kafka_group_name = 'clickhouse'`,
		`kafka_format = 'Protobuf'`,
		fmt.Sprintf(`kafka_schema = 'flow-%s.proto:FlowMessagev%s'`, hash, hash),
		fmt.Sprintf(`kafka_num_consumers = %d`, c.config.Kafka.Consumers),
		`kafka_thread_per_consumer = 1`,
		`kafka_handle_error_mode = 'stream'`,
	}, ", "))

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
		if err := c.d.ClickHouse.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s SYNC`, table)); err != nil {
			return fmt.Errorf("cannot drop %s: %w", table, err)
		}
	}
	if err := c.d.ClickHouse.Exec(ctx, createQuery); err != nil {
		return fmt.Errorf("cannot create raw flows table: %w", err)
	}

	return nil
}

func (c *Component) createRawFlowsConsumerView(ctx context.Context) error {
	tableName := fmt.Sprintf("flows_%s_raw", c.d.Schema.ProtobufMessageHash())
	viewName := fmt.Sprintf("%s_consumer", tableName)

	// Build SELECT query
	selectQuery, err := stemplate(
		`{{ .With }} SELECT {{ .Columns }} FROM {{ .Database }}.{{ .Table }} WHERE length(_error) = 0`,
		gin.H{
			"With": "WITH arrayCompact(DstASPath) AS c_DstASPath",
			"Columns": strings.Join(c.d.Schema.ClickHouseSelectColumns(
				schema.ClickHouseSubstituteGenerates,
				schema.ClickHouseSubstituteTransforms,
				schema.ClickHouseSkipAliasedColumns), ", "),
			"Database": c.config.Database,
			"Table":    tableName,
		})
	if err != nil {
		return fmt.Errorf("cannot build select statement for raw flows consumer view: %w", err)
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
	if err := c.d.ClickHouse.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s SYNC`, viewName)); err != nil {
		return fmt.Errorf("cannot drop table %s: %w", viewName, err)
	}
	if err := c.d.ClickHouse.Exec(ctx,
		fmt.Sprintf("CREATE MATERIALIZED VIEW %s TO flows AS %s",
			viewName, selectQuery)); err != nil {
		return fmt.Errorf("cannot create raw flows consumer view: %w", err)
	}

	return nil
}

func (c *Component) createRawFlowsErrorsView(ctx context.Context) error {
	tableName := fmt.Sprintf("flows_%s_raw", c.d.Schema.ProtobufMessageHash())
	viewName := fmt.Sprintf("%s_errors", tableName)

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
		"Table":    tableName,
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
	if err := c.d.ClickHouse.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s SYNC`, viewName)); err != nil {
		return fmt.Errorf("cannot drop table %s: %w", viewName, err)
	}
	if err := c.d.ClickHouse.Exec(ctx,
		fmt.Sprintf(`
CREATE MATERIALIZED VIEW %s
ENGINE = MergeTree
ORDER BY (timestamp, topic, partition, offset)
PARTITION BY toYYYYMMDDhhmmss(toStartOfHour(timestamp))
TTL timestamp + INTERVAL 1 DAY
AS %s`,
			viewName, selectQuery)); err != nil {
		return fmt.Errorf("cannot create raw flows errors view: %w", err)
	}

	return nil
}

func (c *Component) createOrUpdateFlowsTable(ctx context.Context, resolution ResolutionConfiguration) error {
	var tableName string
	if resolution.Interval == 0 {
		tableName = "flows"
	} else {
		tableName = fmt.Sprintf("flows_%s", resolution.Interval)
	}
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
CREATE TABLE flows ({{ .Schema }})
ENGINE = MergeTree
PARTITION BY toYYYYMMDDhhmmss(toStartOfInterval(TimeReceived, INTERVAL {{ .PartitionInterval }} second))
ORDER BY (TimeReceived, ExporterAddress, InIfName, OutIfName)
TTL TimeReceived + toIntervalSecond({{ .TTL }})
`, gin.H{
				"Schema":            c.d.Schema.ClickHouseCreateTable(),
				"PartitionInterval": partitionInterval,
				"TTL":               ttl,
			})
		} else {
			createQuery, err = stemplate(`
CREATE TABLE {{ .Table }} ({{ .Schema }})
ENGINE = SummingMergeTree((Bytes, Packets))
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
			})
		}
		if err != nil {
			return fmt.Errorf("cannot build create table statement for %s: %w", tableName, err)
		}
		if err := c.d.ClickHouse.Exec(ctx, createQuery); err != nil {
			return fmt.Errorf("cannot create %s: %w", tableName, err)
		}
		return nil
	}

	// Get existing columns
	var existingColumns []struct {
		Name         string `ch:"name"`
		Type         string `ch:"type"`
		IsSortingKey uint8  `ch:"is_in_sorting_key"`
		IsPrimaryKey uint8  `ch:"is_in_primary_key"`
	}
	if err := c.d.ClickHouse.Select(ctx, &existingColumns, `
SELECT name, type, is_in_sorting_key, is_in_primary_key
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
		if resolution.Interval > 0 && wantedColumn.MainOnly {
			continue
		}
		// Check if the column already exists
		for _, existingColumn := range existingColumns {
			if wantedColumn.Name == existingColumn.Name {
				// Do a few sanity checks
				if wantedColumn.ClickHouseType != existingColumn.Type {
					if slices.Contains(c.d.Schema.ClickHousePrimaryKeys(), wantedColumn.Name) {
						return fmt.Errorf("table %s, primary key column %s has a non-matching type: %s vs %s",
							tableName, wantedColumn.Name, existingColumn.Type, wantedColumn.ClickHouseType)
					}
				}
				if resolution.Interval > 0 && slices.Contains(c.d.Schema.ClickHousePrimaryKeys(), wantedColumn.Name) && existingColumn.IsPrimaryKey == 0 {
					return fmt.Errorf("table %s, column %s should be a primary key, cannot change that",
						tableName, wantedColumn.Name)
				}
				if resolution.Interval > 0 && !wantedColumn.ClickHouseNotSortingKey && existingColumn.IsSortingKey == 0 {
					// That's something we can fix, but we need to drop it before recreating it
					err := c.d.ClickHouse.Exec(ctx,
						fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tableName, existingColumn.Name))
					if err != nil {
						return fmt.Errorf("cannot drop %s from %s to fix ordering: %w",
							existingColumn.Name, tableName, err)
					}
					// Schedule adding it back
					modifications = append(modifications,
						fmt.Sprintf("ADD COLUMN %s AFTER %s", wantedColumn.ClickHouseDefinition(), previousColumn))
				} else if wantedColumn.ClickHouseType != existingColumn.Type {
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
			if err := c.d.ClickHouse.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s SYNC`, viewName)); err != nil {
				return fmt.Errorf("cannot drop %s: %w", viewName, err)
			}
		}
		err := c.d.ClickHouse.Exec(ctx, fmt.Sprintf("ALTER TABLE %s %s", tableName, strings.Join(modifications, ", ")))
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
		if err := c.d.ClickHouse.Exec(ctx, fmt.Sprintf("ALTER TABLE %s MODIFY %s", tableName, ttlClause)); err != nil {
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
FROM {{ .Database }}.flows`, gin.H{
		"Database": c.config.Database,
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
	if err := c.d.ClickHouse.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s SYNC`, viewName)); err != nil {
		return fmt.Errorf("cannot drop table %s: %w", viewName, err)
	}
	if err := c.d.ClickHouse.Exec(ctx,
		fmt.Sprintf(`CREATE MATERIALIZED VIEW %s TO %s AS %s`, viewName, tableName, selectQuery)); err != nil {
		return fmt.Errorf("cannot create %s: %w", viewName, err)
	}
	return nil
}
