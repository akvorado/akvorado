package clickhouse

import (
	"context"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"

	"akvorado/common/reporter"
	"akvorado/inlet/flow"
)

const flowSchema = `
 TimeReceived DateTime CODEC(DoubleDelta, LZ4),
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
 Packets UInt64,
 ForwardingStatus UInt32
`

func (c *Component) migrationsStepCreateFlowsTable(resolution ResolutionConfiguration) migrationStepFunc {
	return func(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
		// Unconsolidated flows table
		tableName := "flows"
		do := func() error {
			return conn.Exec(ctx, fmt.Sprintf(`
CREATE TABLE flows (
 Date Date,
%s
)
ENGINE = MergeTree
PARTITION BY Date
ORDER BY TimeReceived`, flowSchema))
		}
		if resolution.Interval != 0 {
			// Consolidated flows table
			tableName = fmt.Sprintf("flows_%s", resolution.Interval)
			viewName := fmt.Sprintf("%s_consumer", tableName)
			// The ORDER BY clause excludes field that are usually
			// deduced from included fields, assuming they won't
			// change for the interval of time considered. It
			// excludes Bytes and Packets that are summed. The
			// order is the one we are most likely to use when
			// filtering.
			do = func() error {
				l.Debug().Msgf("drop flows consumer table for interval %s", resolution.Interval)
				err := conn.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, viewName))
				if err != nil {
					l.Err(err).Msgf("cannot drop flows consumer table for interval %s",
						resolution.Interval)
					return fmt.Errorf("cannot drop flows consumer table for interval %s: %w",
						resolution.Interval, err)
				}
				return conn.Exec(ctx, fmt.Sprintf(`
CREATE TABLE %s (
 Date Date,
%s
)
ENGINE = SummingMergeTree((Bytes, Packets))
PARTITION BY Date
ORDER BY (Date, TimeReceived,
          ExporterAddress,
          EType, Proto,
          InIfName, SrcAS, SrcAddr, SrcPort, ForwardingStatus,
          OutIfName, DstAS, DstAddr, DstPort,
          SamplingRate)`, tableName, flowSchema))
			}
		}
		return migrationStep{
			CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2`,
			Args:       []interface{}{tableName, c.config.Configuration.Database},
			Do:         do,
		}
	}
}

func (c *Component) migrationsStepCreateFlowsConsumerTable(resolution ResolutionConfiguration) migrationStepFunc {
	return func(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
		if resolution.Interval == 0 {
			// Consumer for the flows table are done later.
			return migrationStep{
				CheckQuery: `SELECT 1`,
				Args:       []interface{}{},
				Do:         func() error { return nil },
			}
		}
		tableName := fmt.Sprintf("flows_%s", resolution.Interval)
		viewName := fmt.Sprintf("%s_consumer", tableName)
		// No GROUP BY, we assume the SummingMergeTree will take care of that
		return migrationStep{
			CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2`,
			Args:       []interface{}{viewName, c.config.Configuration.Database},
			Do: func() error {
				return conn.Exec(ctx, fmt.Sprintf(`
CREATE MATERIALIZED VIEW %s TO %s
AS SELECT
 *
REPLACE(toStartOfInterval(TimeReceived, INTERVAL %d second) AS TimeReceived)
FROM %s`, viewName, tableName, uint64(resolution.Interval.Seconds()), "flows"))
			},
		}
	}
}

func (c *Component) migrationsStepSetTTLFlowsTable(resolution ResolutionConfiguration) migrationStepFunc {
	return func(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
		if resolution.TTL == 0 {
			l.Info().Msgf("not changing TTL for flows table with interval %s", resolution.Interval)
			return migrationStep{
				CheckQuery: `SELECT 1`,
				Args:       []interface{}{},
				Do:         func() error { return nil },
			}
		}
		tableName := "flows"
		if resolution.Interval != 0 {
			tableName = fmt.Sprintf("flows_%s", resolution.Interval)
		}
		seconds := uint64(resolution.TTL.Seconds())
		enginePattern := fmt.Sprintf("TTL TimeReceived + toIntervalSecond(%d)", seconds)
		return migrationStep{
			CheckQuery: `
SELECT 1 FROM system.tables
WHERE name = $1 AND database = $2 AND engine_full LIKE $3`,
			Args: []interface{}{
				tableName,
				c.config.Configuration.Database,
				fmt.Sprintf("%% %s %%", enginePattern),
			},
			Do: func() error {
				l.Warn().Msgf("updating TTL of flows table with interval %s, this can take a long time",
					resolution.Interval)
				return conn.Exec(ctx,
					fmt.Sprintf("ALTER TABLE %s MODIFY TTL TimeReceived + toIntervalSecond(%d)",
						tableName,
						seconds))
			},
		}
	}
}

func (c *Component) migrationStepCreateExportersView(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	return migrationStep{
		CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2`,
		Args:       []interface{}{"exporters", c.config.Configuration.Database},
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

func (c *Component) migrationStepCreateProtocolsDictionary(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	protocolsURL := fmt.Sprintf("%s/api/v0/orchestrator/clickhouse/protocols.csv", c.config.OrchestratorURL)
	source := fmt.Sprintf(`SOURCE(HTTP(URL '%s' FORMAT 'CSVWithNames'))`, protocolsURL)
	sourceLike := fmt.Sprintf("%% %s %%", source)
	return migrationStep{
		CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2 AND create_table_query LIKE $3`,
		Args:       []interface{}{"protocols", c.config.Configuration.Database, sourceLike},
		Do: func() error {
			return conn.Exec(ctx, fmt.Sprintf(`
CREATE OR REPLACE DICTIONARY protocols (
 proto UInt8 INJECTIVE,
 name String,
 description String
)
PRIMARY KEY proto
%s
LIFETIME(MIN 0 MAX 3600)
LAYOUT(HASHED())
`, source))
		},
	}
}

func (c *Component) migrationStepCreateASNsDictionary(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	asnsURL := fmt.Sprintf("%s/api/v0/orchestrator/clickhouse/asns.csv", c.config.OrchestratorURL)
	source := fmt.Sprintf(`SOURCE(HTTP(URL '%s' FORMAT 'CSVWithNames'))`, asnsURL)
	sourceLike := fmt.Sprintf("%% %s %%", source)
	return migrationStep{
		CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2 AND create_table_query LIKE $3`,
		Args:       []interface{}{"asns", c.config.Configuration.Database, sourceLike},
		Do: func() error {
			return conn.Exec(ctx, fmt.Sprintf(`
CREATE OR REPLACE DICTIONARY asns (
 asn UInt32 INJECTIVE,
 name String
)

PRIMARY KEY asn
%s
LIFETIME(MIN 0 MAX 3600)
LAYOUT(HASHED())
`, source))
		},
	}

}

func (c *Component) migrationStepCreateRawFlowsTable(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
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
		CheckQuery: `
SELECT bitAnd(v1, v2) FROM (
 SELECT 1 AS v1
 FROM system.tables
 WHERE name = $1
 AND database = $2
 AND engine_full = $3
) t1, (
 SELECT groupBitXor(cityHash64(name,type,position)) == 14541584690055279959 AS v2
 FROM system.columns
 WHERE database = $2
 AND table = $1
) t2
`,
		Args: []interface{}{tableName, c.config.Configuration.Database, kafkaEngine},
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
%s
)
ENGINE = %s`, tableName, flowSchema, kafkaEngine))
		},
	}
}

func (c *Component) migrationStepCreateRawFlowsConsumerView(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	tableName := fmt.Sprintf("flows_%d_raw", flow.CurrentSchemaVersion)
	viewName := fmt.Sprintf("%s_consumer", tableName)
	return migrationStep{
		CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2`,
		Args:       []interface{}{viewName, c.config.Configuration.Database},
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
