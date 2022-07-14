// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"akvorado/common/reporter"
	"akvorado/inlet/flow"
)

const flowsSchema = `
 TimeReceived DateTime CODEC(DoubleDelta, LZ4),
 SamplingRate UInt64,
 ExporterAddress LowCardinality(IPv6),
 ExporterName LowCardinality(String),
 ExporterGroup LowCardinality(String),
 SrcAddr IPv6,
 DstAddr IPv6,
 SrcAS UInt32,
 DstAS UInt32,
 SrcNetName LowCardinality(String),
 DstNetName LowCardinality(String),
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

// partialSchema returns the above schema minus some columns
func partialSchema(remove ...string) string {
	schema := []string{}
outer:
	for _, l := range strings.Split(flowsSchema, "\n") {
		for _, p := range remove {
			if strings.HasPrefix(strings.TrimSpace(l), fmt.Sprintf("%s ", p)) {
				continue outer
			}
		}
		schema = append(schema, l)
	}
	return strings.Join(schema, "\n")
}

var nullMigrationStep = migrationStep{
	CheckQuery: `SELECT 1`,
	Args:       []interface{}{},
	Do:         func() error { return nil },
}

func (c *Component) migrationsStepCreateFlowsTable(resolution ResolutionConfiguration) migrationStepFunc {
	return func(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
		if resolution.Interval == 0 {
			// Unconsolidated flows table
			return migrationStep{
				CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2`,
				Args:       []interface{}{"flows", c.config.Configuration.Database},
				Do: func() error {
					return conn.Exec(ctx, fmt.Sprintf(`
CREATE TABLE flows (
%s
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDDhhmmss(toStartOfInterval(TimeReceived, INTERVAL 6 hour))
ORDER BY (TimeReceived, ExporterAddress, InIfName, OutIfName)`, flowsSchema))
				},
			}
		}
		// Consolidated table. The ORDER BY clause excludes
		// field that are usually deduced from included
		// fields, assuming they won't change for the interval
		// of time considered. It excludes Bytes and Packets
		// that are summed. The order is the one we are most
		// likely to use when filtering. SrcAddr and DstAddr
		// are removed.
		tableName := fmt.Sprintf("flows_%s", resolution.Interval)
		viewName := fmt.Sprintf("%s_consumer", tableName)
		return migrationStep{
			CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2`,
			Args:       []interface{}{tableName, c.config.Configuration.Database},
			Do: func() error {
				l.Debug().Msgf("drop flows consumer table for interval %s", resolution.Interval)
				err := conn.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, viewName))
				if err != nil {
					return fmt.Errorf("cannot drop flows consumer table for interval %s: %w",
						resolution.Interval, err)
				}

				partitionInterval := uint64((resolution.TTL / time.Duration(c.config.MaxPartitions)).Seconds())
				// Primary key does not cover all the sorting key as we cannot modify it.
				return conn.Exec(ctx, fmt.Sprintf(`
CREATE TABLE %s (
%s
)
ENGINE = SummingMergeTree((Bytes, Packets))
PARTITION BY toYYYYMMDDhhmmss(toStartOfInterval(TimeReceived, INTERVAL %d second))
PRIMARY KEY (TimeReceived,
          ExporterAddress,
          EType, Proto,
          InIfName, SrcAS, ForwardingStatus,
          OutIfName, DstAS,
          SamplingRate)
ORDER BY (TimeReceived,
          ExporterAddress,
          EType, Proto,
          InIfName, SrcAS, ForwardingStatus,
          OutIfName, DstAS,
          SamplingRate,
          SrcNetName, DstNetName)`,
					tableName,
					partialSchema("SrcAddr", "DstAddr", "SrcPort", "DstPort"),
					partitionInterval))
			},
		}
	}
}

func (c *Component) migrationStepAddPacketSizeBucketColumn(resolution ResolutionConfiguration) migrationStepFunc {
	return func(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
		var tableName string
		if resolution.Interval == 0 {
			tableName = "flows"
		} else {
			tableName = fmt.Sprintf("flows_%s", resolution.Interval)
		}
		return migrationStep{
			CheckQuery: `SELECT 1 FROM system.columns WHERE table = $1 AND database = $2 AND name = $3`,
			Args:       []interface{}{tableName, c.config.Configuration.Database, "PacketSizeBucket"},
			Do: func() error {
				return conn.Exec(ctx, fmt.Sprintf(`
ALTER TABLE %s ADD COLUMN PacketSize UInt64 ALIAS intDiv(Bytes, Packets) AFTER Packets,
               ADD COLUMN PacketSizeBucket LowCardinality(String) ALIAS multiIf(PacketSize < 64, '0-63', PacketSize < 128, '64-127', PacketSize < 256, '128-255', PacketSize < 512, '256-511', PacketSize < 768, '512-767', PacketSize < 1024, '768-1023', PacketSize < 1280, '1024-1279', PacketSize < 1501, '1280-1500', PacketSize < 2048, '1501-2047', PacketSize < 3072, '2048-3071', PacketSize < 4096, '3072-4095', PacketSize < 8192, '4096-8191', PacketSize < 10240, '8192-10239', PacketSize < 16384, '10240-16383', PacketSize < 32768, '16384-32767', PacketSize < 65536, '32768-65535', '65536-Inf') AFTER PacketSize`,
					tableName))
			},
		}
	}
}

func (c *Component) migrationStepAddSrcNetNameDstNetNameColumns(resolution ResolutionConfiguration) migrationStepFunc {
	return func(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
		var tableName string
		if resolution.Interval == 0 {
			tableName = "flows"
		} else {
			tableName = fmt.Sprintf("flows_%s", resolution.Interval)
		}
		return migrationStep{
			CheckQuery: `SELECT 1 FROM system.columns WHERE table = $1 AND database = $2 AND name = $3`,
			Args:       []interface{}{tableName, c.config.Configuration.Database, "DstNetName"},
			Do: func() error {
				modifications := []string{
					`ADD COLUMN SrcNetName LowCardinality(String) AFTER DstAS`,
					`ADD COLUMN DstNetName LowCardinality(String) AFTER SrcNetName`,
				}
				if tableName != "flows" {
					modifications = append(modifications,
						`MODIFY ORDER BY (TimeReceived, ExporterAddress, EType, Proto,
                                                                  InIfName, SrcAS, ForwardingStatus,
                                                                  OutIfName, DstAS, SamplingRate,
                                                                  SrcNetName, DstNetName)`)
				}
				return conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s %s`,
					tableName, strings.Join(modifications, ", ")))
			},
		}
	}
}

func (c *Component) migrationsStepCreateFlowsConsumerTable(resolution ResolutionConfiguration) migrationStepFunc {
	return func(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
		if resolution.Interval == 0 {
			// Consumer for the flows table are done later.
			return nullMigrationStep
		}
		tableName := fmt.Sprintf("flows_%s", resolution.Interval)
		viewName := fmt.Sprintf("%s_consumer", tableName)
		return migrationStep{
			CheckQuery: `
SELECT bitAnd(v1, v2) FROM (
 SELECT 1 AS v1
 FROM system.tables WHERE name = $1 AND database = $2
) t1, (
 SELECT groupBitXor(cityHash64(name,type,position)) == 13289045892922565912 AS v2
 FROM system.columns
 WHERE database = $2
 AND table = $1
) t2`,
			Args: []interface{}{viewName, c.config.Configuration.Database},
			// No GROUP BY, the SummingMergeTree will take care of that
			Do: func() error {
				l.Debug().Msg("drop consumer table")
				err := conn.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, viewName))
				if err != nil {
					return fmt.Errorf("cannot drop consumer table: %w", err)
				}
				l.Debug().Msg("create consumer table")
				return conn.Exec(ctx, fmt.Sprintf(`
CREATE MATERIALIZED VIEW %s TO %s
AS SELECT
 *
EXCEPT(SrcAddr, DstAddr, SrcPort, DstPort)
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
		ttl := fmt.Sprintf("TTL TimeReceived + toIntervalSecond(%d)", seconds)
		return migrationStep{
			CheckQuery: `
SELECT 1 FROM system.tables
WHERE name = $1 AND database = $2 AND engine_full LIKE $3`,
			Args: []interface{}{
				tableName,
				c.config.Configuration.Database,
				fmt.Sprintf("%% %s %%", ttl),
			},
			Do: func() error {
				l.Warn().Msgf("updating TTL of flows table with interval %s, this can take a long time",
					resolution.Interval)
				return conn.Exec(ctx, fmt.Sprintf("ALTER TABLE %s MODIFY %s", tableName, ttl))
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
	settings := `SETTINGS(format_csv_allow_single_quotes = 0)`
	sourceLike := fmt.Sprintf("%% %s%% %s%%", source, settings)
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
%s
`, source, settings))
		},
	}
}

func (c *Component) migrationStepCreateASNsDictionary(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	asnsURL := fmt.Sprintf("%s/api/v0/orchestrator/clickhouse/asns.csv", c.config.OrchestratorURL)
	source := fmt.Sprintf(`SOURCE(HTTP(URL '%s' FORMAT 'CSVWithNames'))`, asnsURL)
	settings := `SETTINGS(format_csv_allow_single_quotes = 0)`
	sourceLike := fmt.Sprintf("%% %s%% %s%%", source, settings)
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
%s
`, source, settings))
		},
	}

}

func (c *Component) migrationStepCreateNetworksDictionary(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	networksURL := fmt.Sprintf("%s/api/v0/orchestrator/clickhouse/networks.csv", c.config.OrchestratorURL)
	source := fmt.Sprintf(`SOURCE(HTTP(URL '%s' FORMAT 'CSVWithNames'))`, networksURL)
	settings := `SETTINGS(format_csv_allow_single_quotes = 0)`
	sourceLike := fmt.Sprintf("%% %s%% %s%%", source, settings)
	return migrationStep{
		CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2 AND create_table_query LIKE $3`,
		Args:       []interface{}{"networks", c.config.Configuration.Database, sourceLike},
		Do: func() error {
			return conn.Exec(ctx, fmt.Sprintf(`
CREATE OR REPLACE DICTIONARY networks (
 network String,
 name String
)

PRIMARY KEY network
%s
LIFETIME(MIN 0 MAX 3600)
LAYOUT(IP_TRIE())
%s
`, source, settings))
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
				return fmt.Errorf("cannot drop raw consumer table: %w", err)
			}
			l.Debug().Msg("drop raw table")
			err = conn.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, tableName))
			if err != nil {
				return fmt.Errorf("cannot drop raw table: %w", err)
			}
			l.Debug().Msg("create raw table")
			return conn.Exec(ctx, fmt.Sprintf(`
CREATE TABLE %s
(
%s
)
ENGINE = %s`, tableName, partialSchema("SrcNetName", "DstNetName"), kafkaEngine))
		},
	}
}

func (c *Component) migrationStepCreateRawFlowsConsumerView(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	tableName := fmt.Sprintf("flows_%d_raw", flow.CurrentSchemaVersion)
	viewName := fmt.Sprintf("%s_consumer", tableName)
	return migrationStep{
		CheckQuery: `
SELECT bitAnd(v1, v2) FROM (
 SELECT 1 AS v1
 FROM system.tables WHERE name = $1 AND database = $2
) t1, (
 SELECT groupBitXor(cityHash64(name,type,position)) == 17364559455632379339 AS v2
 FROM system.columns
 WHERE database = $2
 AND table = $1
) t2`,
		Args: []interface{}{viewName, c.config.Configuration.Database},
		Do: func() error {
			l.Debug().Msg("drop consumer table")
			err := conn.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, viewName))
			if err != nil {
				return fmt.Errorf("cannot drop consumer table: %w", err)
			}
			l.Debug().Msg("create consumer table")
			return conn.Exec(ctx, fmt.Sprintf(`
CREATE MATERIALIZED VIEW %s TO flows
AS SELECT
 *,
 dictGetOrDefault('networks', 'name', SrcAddr, '') AS SrcNetName,
 dictGetOrDefault('networks', 'name', DstAddr, '') AS DstNetName
FROM %s`, viewName, tableName))
		},
	}
}
