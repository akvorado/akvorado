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

const (
	// flowsSchema is the canonical schema for flows table
	flowsSchema = `
 TimeReceived DateTime CODEC(DoubleDelta, LZ4),
 SamplingRate UInt64,
 ExporterAddress LowCardinality(IPv6),
 ExporterName LowCardinality(String),
 ExporterGroup LowCardinality(String),
 ExporterRole LowCardinality(String),
 ExporterSite LowCardinality(String),
 ExporterRegion LowCardinality(String),
 ExporterTenant LowCardinality(String),
 SrcAddr IPv6,
 DstAddr IPv6,
 SrcAS UInt32,
 DstAS UInt32,
 SrcNetName LowCardinality(String),
 DstNetName LowCardinality(String),
 SrcNetRole LowCardinality(String),
 DstNetRole LowCardinality(String),
 SrcNetSite LowCardinality(String),
 DstNetSite LowCardinality(String),
 SrcNetRegion LowCardinality(String),
 DstNetRegion LowCardinality(String),
 SrcNetTenant LowCardinality(String),
 DstNetTenant LowCardinality(String),
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
)

// queryTableHash can be used to check if a table exists with the specified schema.
func queryTableHash(hash uint64, more string) string {
	return fmt.Sprintf(`
SELECT bitAnd(v1, v2) FROM (
 SELECT 1 AS v1
 FROM system.tables
 WHERE name = $1 AND database = currentDatabase() %s
) t1, (
 SELECT groupBitXor(cityHash64(name,type,position)) == %d AS v2
 FROM system.columns
 WHERE table = $1 AND database = currentDatabase()
) t2`, more, hash)
}

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
			partitionInterval := uint64((resolution.TTL / time.Duration(c.config.MaxPartitions)).Seconds())
			return migrationStep{
				CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = currentDatabase()`,
				Args:       []interface{}{"flows"},
				Do: func() error {
					return conn.Exec(ctx, fmt.Sprintf(`
CREATE TABLE flows (
%s
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDDhhmmss(toStartOfInterval(TimeReceived, INTERVAL %d second))
ORDER BY (TimeReceived, ExporterAddress, InIfName, OutIfName)`, flowsSchema, partitionInterval))
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
			CheckQuery: `SELECT 1 FROM system.tables WHERE name = $1 AND database = currentDatabase()`,
			Args:       []interface{}{tableName},
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
          SrcNetName, DstNetName,
          SrcNetRole, DstNetRole,
          SrcNetSite, DstNetSite,
          SrcNetRegion, DstNetRegion,
          SrcNetTenant, DstNetTenant)`,
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
			CheckQuery: `
SELECT 1 FROM system.columns
WHERE table = $1 AND database = currentDatabase() AND name = $2`,
			Args: []interface{}{tableName, "PacketSizeBucket"},
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
			CheckQuery: `
SELECT 1 FROM system.columns
WHERE table = $1 AND database = currentDatabase() AND name = $2`,
			Args: []interface{}{tableName, "DstNetName"},
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

func (c *Component) migrationStepAddSrcNetNameDstNetOthersColumns(resolution ResolutionConfiguration) migrationStepFunc {
	return func(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
		var tableName string
		if resolution.Interval == 0 {
			tableName = "flows"
		} else {
			tableName = fmt.Sprintf("flows_%s", resolution.Interval)
		}
		return migrationStep{
			CheckQuery: `
SELECT 1 FROM system.columns
WHERE table = $1 AND database = currentDatabase() AND name = $2`,
			Args: []interface{}{tableName, "DstNetRole"},
			Do: func() error {
				modifications := []string{
					`ADD COLUMN SrcNetRole LowCardinality(String) AFTER DstNetName`,
					`ADD COLUMN DstNetRole LowCardinality(String) AFTER SrcNetRole`,
					`ADD COLUMN SrcNetSite LowCardinality(String) AFTER DstNetRole`,
					`ADD COLUMN DstNetSite LowCardinality(String) AFTER SrcNetSite`,
					`ADD COLUMN SrcNetRegion LowCardinality(String) AFTER DstNetSite`,
					`ADD COLUMN DstNetRegion LowCardinality(String) AFTER SrcNetRegion`,
					`ADD COLUMN SrcNetTenant LowCardinality(String) AFTER DstNetRegion`,
					`ADD COLUMN DstNetTenant LowCardinality(String) AFTER SrcNetTenant`,
				}
				if tableName != "flows" {
					modifications = append(modifications,
						`MODIFY ORDER BY (TimeReceived, ExporterAddress, EType, Proto,
                                                                  InIfName, SrcAS, ForwardingStatus,
                                                                  OutIfName, DstAS, SamplingRate,
                                                                  SrcNetName, DstNetName,
                                                                  SrcNetRole, DstNetRole,
                                                                  SrcNetSite, DstNetSite,
                                                                  SrcNetRegion, DstNetRegion,
                                                                  SrcNetTenant, DstNetTenant)`)
				}
				return conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s %s`,
					tableName, strings.Join(modifications, ", ")))
			},
		}
	}
}

func (c *Component) migrationStepAddExporterColumns(resolution ResolutionConfiguration) migrationStepFunc {
	return func(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
		var tableName string
		if resolution.Interval == 0 {
			tableName = "flows"
		} else {
			tableName = fmt.Sprintf("flows_%s", resolution.Interval)
		}
		return migrationStep{
			CheckQuery: `
SELECT 1 FROM system.columns
WHERE table = $1 AND database = currentDatabase() AND name = $2`,
			Args: []interface{}{tableName, "ExporterTenant"},
			Do: func() error {
				modifications := []string{
					`ADD COLUMN ExporterRole LowCardinality(String) AFTER ExporterGroup`,
					`ADD COLUMN ExporterSite LowCardinality(String) AFTER ExporterRole`,
					`ADD COLUMN ExporterRegion LowCardinality(String) AFTER ExporterSite`,
					`ADD COLUMN ExporterTenant LowCardinality(String) AFTER ExporterRegion`,
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
			CheckQuery: queryTableHash(7356168458686845598, ""),
			Args:       []interface{}{viewName},
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
WHERE name = $1 AND database = currentDatabase() AND engine_full LIKE $2`,
			Args: []interface{}{
				tableName,
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
		CheckQuery: queryTableHash(9989732154180416521, ""),
		Args:       []interface{}{"exporters"},
		Do: func() error {
			l.Debug().Msg("drop exporters table")
			err := conn.Exec(ctx, `DROP TABLE IF EXISTS exporters`)
			if err != nil {
				return fmt.Errorf("cannot drop exporters table: %w", err)
			}
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
 ExporterRole,
 ExporterSite,
 ExporterRegion,
 ExporterTenant,
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
		CheckQuery: `
SELECT 1 FROM system.tables
 WHERE name = $1 AND database = currentDatabase() AND create_table_query LIKE $2`,
		Args: []interface{}{"protocols", sourceLike},
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
		CheckQuery: `
SELECT 1 FROM system.tables
WHERE name = $1 AND database = currentDatabase() AND create_table_query LIKE $2`,
		Args: []interface{}{"asns", sourceLike},
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
		CheckQuery: queryTableHash(5246378884861475308, "AND create_table_query LIKE $2"),
		Args:       []interface{}{"networks", sourceLike},
		Do: func() error {
			return conn.Exec(ctx, fmt.Sprintf(`
CREATE OR REPLACE DICTIONARY networks (
 network String,
 name String,
 role String,
 site String,
 region String,
 tenant String
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
		fmt.Sprintf(`kafka_schema = 'flow-%d.proto:FlowMessagev%d',`,
			flow.CurrentSchemaVersion, flow.CurrentSchemaVersion),
		fmt.Sprintf(`kafka_num_consumers = %d,`, c.config.Kafka.Consumers),
		`kafka_thread_per_consumer = 1`,
	}, " ")
	return migrationStep{
		CheckQuery: queryTableHash(4229371004936784880, "AND engine_full = $2"),
		Args:       []interface{}{tableName, kafkaEngine},
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
ENGINE = %s`, tableName, partialSchema(
				"SrcNetName", "DstNetName",
				"SrcNetRole", "DstNetRole",
				"SrcNetSite", "DstNetSite",
				"SrcNetRegion", "DstNetRegion",
				"SrcNetTenant", "DstNetTenant",
			), kafkaEngine))
		},
	}
}

func (c *Component) migrationStepCreateRawFlowsConsumerView(ctx context.Context, l reporter.Logger, conn clickhouse.Conn) migrationStep {
	tableName := fmt.Sprintf("flows_%d_raw", flow.CurrentSchemaVersion)
	viewName := fmt.Sprintf("%s_consumer", tableName)
	return migrationStep{
		CheckQuery: queryTableHash(17295069153939039375, ""),
		Args:       []interface{}{viewName},
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
 dictGetOrDefault('networks', 'name', DstAddr, '') AS DstNetName,
 dictGetOrDefault('networks', 'role', SrcAddr, '') AS SrcNetRole,
 dictGetOrDefault('networks', 'role', DstAddr, '') AS DstNetRole,
 dictGetOrDefault('networks', 'site', SrcAddr, '') AS SrcNetSite,
 dictGetOrDefault('networks', 'site', DstAddr, '') AS DstNetSite,
 dictGetOrDefault('networks', 'region', SrcAddr, '') AS SrcNetRegion,
 dictGetOrDefault('networks', 'region', DstAddr, '') AS DstNetRegion,
 dictGetOrDefault('networks', 'tenant', SrcAddr, '') AS SrcNetTenant,
 dictGetOrDefault('networks', 'tenant', DstAddr, '') AS DstNetTenant
FROM %s`, viewName, tableName))
		},
	}
}
