package clickhouse

import (
	"akvorado/common/reporter"
	"akvorado/inlet/flow"
	"context"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func (c *Component) migrateStepCreateFlowsTable(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	return migrationStep{
		CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2`,
		Args:       []interface{}{"flows", c.config.Database},
		Do: func() error {
			return conn.Exec(ctx, `
CREATE TABLE flows (
 Date Date,
 TimeReceived DateTime CODEC(DoubleDelta, LZ4),
 SequenceNum UInt32,
 SamplingRate UInt64,
 ExporterAddress LowCardinality(IPv6),
 ExporterName LowCardinality(String),
 ExporterGroup LowCardinality(String),
 SrcAddr IPv6,
 DstAddr IPv6,
 SrcAS UInt32,
 DstAS UInt32,
 SrcCountry FixedString(2),
 DstCountry FixedString(2),
 InIfName LowCardinality(String),
 OutIfName LowCardinality(String),
 InIfDescription String,
 OutIfDescription String,
 InIfSpeed UInt32,
 OutIfSpeed UInt32,
 InIfConnectivity LowCardinality(String),
 OutIfConnectivity LowCardinality(String),
 InIfProvider LowCardinality(String),
 OutIfProvider LowCardinality(String),
 InIfBoundary Enum8('undefined' = 0, 'external' = 1, 'internal' = 2),
 OutIfBoundary Enum8('undefined' = 0, 'external' = 1, 'internal' = 2),
 EType UInt32,
 Proto UInt32,
 SrcPort UInt32,
 DstPort UInt32,
 Bytes UInt64,
 Packets UInt64
)
ENGINE = MergeTree
PARTITION BY Date
ORDER BY TimeReceived`)
		},
	}
}

func (c *Component) migrateStepCreateExportersView(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	return migrationStep{
		CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2`,
		Args:       []interface{}{"exporters", c.config.Database},
		Do: func() error {
			return conn.Exec(ctx, `
CREATE MATERIALIZED VIEW exporters
ENGINE = ReplacingMergeTree(TimeReceived)
ORDER BY (ExporterAddress, IfName)
AS
SELECT DISTINCT
 TimeReceived,
 ExporterAddress,
 ExporterName,
 ExporterGroup,
 [InIfName, OutIfName][num] AS IfName,
 [InIfDescription, OutIfDescription][num] AS IfDescription,
 [InIfSpeed, OutIfSpeed][num] AS IfSpeed,
 [InIfConnectivity, OutIfConnectivity][num] AS IfConnectivity,
 [InIfProvider, OutIfProvider][num] AS IfProvider,
 [InIfBoundary, OutIfBoundary][num] AS IfBoundary
FROM flows
ARRAY JOIN arrayEnumerate([1,2]) AS num
`)
		},
	}
}

func (c *Component) migrateStepCreateProtocolsDictionary(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	protocolsURL := fmt.Sprintf("%s/api/v0/configure/clickhouse/protocols.csv", c.config.AkvoradoURL)
	return migrationStep{
		CheckQuery: `SELECT 1 FROM system.dictionaries WHERE name = $1 AND database = $2 AND source = $3`,
		Args:       []interface{}{"protocols", c.config.Database, protocolsURL},
		Do: func() error {
			return conn.Exec(ctx, `
CREATE OR REPLACE DICTIONARY protocols (
 proto UInt8 INJECTIVE,
 name String,
 description String
)
PRIMARY KEY proto
SOURCE(HTTP(URL $1 FORMAT 'CSVWithNames'))
LIFETIME(MIN 0 MAX 3600)
LAYOUT(HASHED())
`, protocolsURL)
		},
	}
}

func (c *Component) migrateStepCreateASNsDictionary(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	asnsURL := fmt.Sprintf("%s/api/v0/configure/clickhouse/asns.csv", c.config.AkvoradoURL)
	return migrationStep{
		CheckQuery: `SELECT 1 FROM system.dictionaries WHERE name = $1 AND database = $2 AND source = $3`,
		Args:       []interface{}{"asns", c.config.Database, asnsURL},
		Do: func() error {
			return conn.Exec(ctx, `
CREATE OR REPLACE DICTIONARY asns (
 asn UInt32 INJECTIVE,
 name String
)
PRIMARY KEY asn
SOURCE(HTTP(URL $1 FORMAT 'CSVWithNames'))
LIFETIME(MIN 0 MAX 3600)
LAYOUT(HASHED())
`, asnsURL)
		},
	}

}

func (c *Component) migrateStepCreateRawFlowsTable(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	tableName := fmt.Sprintf("flows_%d_raw", flow.CurrentSchemaVersion)
	kafkaEngine := strings.Join([]string{
		`Kafka SETTINGS`,
		fmt.Sprintf(`kafka_broker_list = '%s',`,
			strings.Join(c.config.Kafka.Brokers, ",")),
		fmt.Sprintf(`kafka_topic_list = '%s-v%d',`,
			c.config.Kafka.Topic, flow.CurrentSchemaVersion),
		`kafka_group_name = 'clickhouse',`,
		`kafka_format = 'Protobuf',`,
		fmt.Sprintf(`kafka_schema = 'flow-%d.proto:FlowMessage',`,
			flow.CurrentSchemaVersion),
		fmt.Sprintf(`kafka_num_consumers = %d,`, c.config.Kafka.Consumers),
		`kafka_thread_per_consumer = 1`,
	}, " ")
	return migrationStep{
		CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2 AND engine_full = $3`,
		Args:       []interface{}{tableName, c.config.Database, kafkaEngine},
		Do: func() error {
			l.Debug().Msg("drop raw consumer table")
			err := conn.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s_consumer`, tableName))
			if err != nil {
				l.Err(err).Msg("cannot drop raw consumer table")
				return fmt.Errorf("cannot drop raw consumer table: %w", err)
			}
			l.Debug().Msg("drop raw table")
			err = conn.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, tableName))
			if err != nil {
				l.Err(err).Msg("cannot drop raw table")
				return fmt.Errorf("cannot drop raw table: %w", err)
			}
			l.Debug().Msg("create raw table")
			return conn.Exec(ctx, fmt.Sprintf(`
CREATE TABLE %s
(
    TimeReceived UInt64,
    TimeFlowStart UInt64,
    SequenceNum UInt32,
    SamplingRate UInt64,
    ExporterAddress LowCardinality(FixedString(16)),
    ExporterName LowCardinality(String),
    ExporterGroup LowCardinality(String),
    SrcAddr FixedString(16),
    DstAddr FixedString(16),
    SrcAS UInt32,
    DstAS UInt32,
    SrcCountry FixedString(2),
    DstCountry FixedString(2),
    InIfName LowCardinality(String),
    OutIfName LowCardinality(String),
    InIfDescription String,
    OutIfDescription String,
    InIfSpeed UInt32,
    OutIfSpeed UInt32,
    InIfConnectivity LowCardinality(String),
    OutIfConnectivity LowCardinality(String),
    InIfProvider LowCardinality(String),
    OutIfProvider LowCardinality(String),
    InIfBoundary Enum8('undefined' = 0, 'external' = 1, 'internal' = 2),
    OutIfBoundary Enum8('undefined' = 0, 'external' = 1, 'internal' = 2),
    EType UInt32,
    Proto UInt32,
    SrcPort UInt32,
    DstPort UInt32,
    Bytes UInt64,
    Packets UInt64
)
ENGINE = %s`, tableName, kafkaEngine))
		},
	}
}

func (c *Component) migrateStepCreateRawFlowsConsumerView(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	tableName := fmt.Sprintf("flows_%d_raw", flow.CurrentSchemaVersion)
	viewName := fmt.Sprintf("%s_consumer", tableName)
	return migrationStep{
		CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2`,
		Args:       []interface{}{viewName, c.config.Database},
		Do: func() error {
			return conn.Exec(ctx, fmt.Sprintf(`
CREATE MATERIALIZED VIEW %s TO flows
AS SELECT
 toDate(TimeReceived) AS Date,
 *
FROM %s`, viewName, tableName))
		},
	}
}

func (c *Component) migrateStepDropSchemaMigrationsTable(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	return migrationStep{
		CheckQuery: `SELECT COUNT(*) == 0 FROM system.tables WHERE name = $1 AND database = $2`,
		Args:       []interface{}{"schema_migrations", c.config.Database},
		Do: func() error {
			return conn.Exec(ctx, "DROP TABLE schema_migrations")
		},
	}
}
