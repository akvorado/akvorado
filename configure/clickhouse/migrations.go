package clickhouse

import (
	"akvorado/inlet/flow"
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type migrationStep struct {
	// Description of the step
	Description string
	// CheckQuery to execute to check if the step is needed.
	CheckQuery string
	// Arguments to use for the query
	Args []interface{}
	// Function to execute if the query returns no row or returns `0'.
	Do func() error
}

// migrateDatabase execute database migration
func (c *Component) migrateDatabase() error {
	baseURL := c.config.AkvoradoURL
	if baseURL == "" {
		var err error
		if baseURL, err = c.getHTTPBaseURL(c.config.Servers[0]); err != nil {
			return err
		}
	}

	l := c.r.With().
		Str("server", strings.Join(c.config.Servers, ",")).
		Str("database", c.config.Database).
		Str("username", c.config.Username).
		Logger()
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: c.config.Servers,
		Auth: clickhouse.Auth{
			Database: c.config.Database,
			Username: c.config.Username,
			Password: c.config.Password,
		},
	})
	if err != nil {
		l.Err(err).Msg("unable to connect to ClickHouse")
		return fmt.Errorf("unable to connect to ClickHouse: %w", err)
	}

	ctx := c.t.Context(context.Background())
	steps := []migrationStep{
		{
			Description: "create flows table",
			CheckQuery:  `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2`,
			Args:        []interface{}{"flows", c.config.Database},
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
		}, {
			Description: "create exporters view",
			CheckQuery:  `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2`,
			Args:        []interface{}{"exporters", c.config.Database},
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
		}, func() migrationStep {
			protocolsURL := fmt.Sprintf("%s/api/v0/clickhouse/protocols.csv", baseURL)
			return migrationStep{
				Description: "create protocols dictionary",
				CheckQuery: `
SELECT 1 FROM system.dictionaries
WHERE name = $1 AND database = $2 AND source = $3`,
				Args: []interface{}{
					"protocols",
					c.config.Database,
					protocolsURL,
				},
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
		}(), func() migrationStep {
			asnsURL := fmt.Sprintf("%s/api/v0/clickhouse/asns.csv", baseURL)
			return migrationStep{
				Description: "create asns dictionary",
				CheckQuery: `
SELECT 1 FROM system.dictionaries
 WHERE name = $1 AND database = $2 AND source = $3`,
				Args: []interface{}{
					"asns",
					c.config.Database,
					asnsURL,
				},
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
		}(), func() migrationStep {
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
				fmt.Sprintf(`kafka_num_consumers = %d,`, c.config.KafkaThreads),
				`kafka_thread_per_consumer = 1`,
			}, " ")
			return migrationStep{
				Description: "create raw flows table",
				CheckQuery: `
SELECT 1 FROM system.tables
WHERE name = $1 AND database = $2 AND engine_full = $3`,
				Args: []interface{}{tableName, c.config.Database, kafkaEngine},
				Do: func() error {
					l.Debug().Msg("drop raw consumer table")
					err = conn.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s_consumer`, tableName))
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
		}(), func() migrationStep {
			tableName := fmt.Sprintf("flows_%d_raw", flow.CurrentSchemaVersion)
			viewName := fmt.Sprintf("%s_consumer", tableName)
			return migrationStep{
				Description: "create raw flows consumer view",
				CheckQuery:  `SELECT 1 FROM system.tables WHERE name = $1 AND database = $2`,
				Args:        []interface{}{viewName, c.config.Database},
				Do: func() error {
					return conn.Exec(ctx, fmt.Sprintf(`
CREATE MATERIALIZED VIEW %s TO flows
AS SELECT
 toDate(TimeReceived) AS Date,
 *
FROM %s`, viewName, tableName))
				},
			}
		}(), {
			Description: "drop schema_migrations table",
			CheckQuery:  `SELECT COUNT(*) == 0 FROM system.tables WHERE name = $1 AND database = $2`,
			Args:        []interface{}{"schema_migrations", c.config.Database},
			Do: func() error {
				return conn.Exec(ctx, "DROP TABLE schema_migrations")
			},
		},
	}

	count := 0
	total := 0
	for _, step := range steps {
		total++
		l := l.With().Str("step", step.Description).Logger()
		l.Debug().Msg("checking migration step")
		rows, err := conn.Query(ctx, step.CheckQuery, step.Args...)
		if err != nil {
			l.Err(err).Msg("cannot execute check")
			return fmt.Errorf("cannot execute check: %w", err)
		}
		if rows.Next() {
			var val uint8
			if err := rows.Scan(&val); err != nil {
				rows.Close()
				l.Err(err).Msg("cannot parse check result")
				return fmt.Errorf("cannot parse check result: %w", err)
			}
			if val != 0 {
				rows.Close()
				l.Debug().Msg("result not equal to 0, skipping step")
				continue
			}
		}
		rows.Close()
		if err := step.Do(); err != nil {
			l.Err(err).Msg("cannot execute migration step")
			return fmt.Errorf("during migration step: %w", err)
		}
		l.Info().Msg("migration step executed successfully")
		count++
	}

	if count == 0 {
		l.Debug().Msg("no migration needed")
	} else {
		l.Info().Msg("migrations done")
	}
	close(c.migrationsDone)
	c.metrics.migrationsRunning.Set(0)
	c.metrics.migrationsVersion.Set(float64(total))

	return nil
}

// getHTTPBaseURL tries to guess the appropriate URL to access our
// HTTP daemon. It tries to get our IP address using an unconnected
// UDP socket.
func (c *Component) getHTTPBaseURL(address string) (string, error) {
	// Get IP address
	conn, err := net.Dial("udp", address)
	if err != nil {
		return "", fmt.Errorf("cannot get our IP address: %w", err)
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)

	// Combine with HTTP port
	_, port, err := net.SplitHostPort(c.d.HTTP.Address.String())
	if err != nil {
		return "", fmt.Errorf("cannot get HTTP port: %w", err)
	}
	base := fmt.Sprintf("http://%s",
		net.JoinHostPort(localAddr.IP.String(), port))
	c.r.Debug().Msgf("detected base URL is %s", base)
	return base, nil
}
