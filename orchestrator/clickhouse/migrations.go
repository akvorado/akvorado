// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"fmt"
	"net"

	"github.com/ClickHouse/clickhouse-go/v2"

	"akvorado/common/reporter"
)

type migrationStep struct {
	// CheckQuery to execute to check if the step is needed.
	CheckQuery string
	// Arguments to use for the query
	Args []interface{}
	// Function to execute if the query returns no row or returns `0'.
	Do func() error
}

type migrationStepFunc func(context.Context, reporter.Logger, clickhouse.Conn) migrationStep

type migrationStepWithDescription struct {
	Description string
	Step        migrationStepFunc
}

// migrateDatabase execute database migration
func (c *Component) migrateDatabase() error {
	ctx := c.t.Context(nil)

	// Set orchestrator URL
	if c.config.OrchestratorURL == "" {
		baseURL, err := c.getHTTPBaseURL("1.1.1.1:80")
		if err != nil {
			return err
		}
		c.config.OrchestratorURL = baseURL
	}

	// Limit number of consumers to the number of threads
	row := c.d.ClickHouse.QueryRow(ctx, `SELECT getSetting('max_threads')`)
	if err := row.Err(); err != nil {
		c.r.Err(err).Msg("unable to query database")
		return fmt.Errorf("unable to query database: %w", err)
	}
	var threads uint8
	if err := row.Scan(&threads); err != nil {
		c.r.Err(err).Msg("unable to parse number of threads")
		return fmt.Errorf("unable to parse number of threads: %w", err)
	}
	if c.config.Kafka.Consumers > int(threads) {
		c.r.Warn().Msgf("too many consumers requested, capping to %d", threads)
		c.config.Kafka.Consumers = int(threads)
	}

	steps := []migrationStepWithDescription{
		{"create protocols dictionary", c.migrationStepCreateProtocolsDictionary},
		{"create asns dictionary", c.migrationStepCreateASNsDictionary},
		{"create networks dictionary", c.migrationStepCreateNetworksDictionary},
	}
	for _, resolution := range c.config.Resolutions {
		steps = append(steps, []migrationStepWithDescription{
			{
				fmt.Sprintf("create flows table with resolution %s", resolution.Interval),
				c.migrationsStepCreateFlowsTable(resolution),
			}, {
				fmt.Sprintf("add PacketSizeBucket to flows table with resolution %s", resolution.Interval),
				c.migrationStepAddPacketSizeBucketColumn(resolution),
			}, {
				fmt.Sprintf("add SrcNetName/DstNetName to flows table with resolution %s", resolution.Interval),
				c.migrationStepAddSrcNetNameDstNetNameColumns(resolution),
			}, {
				fmt.Sprintf("add SrcNet*/DstNet* to flows table with resolution %s", resolution.Interval),
				c.migrationStepAddSrcNetNameDstNetOthersColumns(resolution),
			}, {
				fmt.Sprintf("add Exporter* to flows table with resolution %s", resolution.Interval),
				c.migrationStepAddExporterColumns(resolution),
			}, {
				fmt.Sprintf("add SrcCountry/DstCountry to ORDER BY for resolution %s", resolution.Interval),
				c.migrationStepFixOrderByCountry(resolution),
			}, {
				fmt.Sprintf("add DstASPath columns to flows table with resolution %s", resolution.Interval),
				c.migrationStepAddDstASPathColumns(resolution),
			},
		}...)
		if resolution.Interval == 0 {
			steps = append(steps, migrationStepWithDescription{
				"add DstCommunities column to flows table",
				c.migrationStepAddDstCommunitiesColumn,
			}, migrationStepWithDescription{
				"add DstLargeCommunities column to flows table",
				c.migrationStepAddDstLargeCommunitiesColumn,
			})
		}
		steps = append(steps, []migrationStepWithDescription{
			{
				fmt.Sprintf("create flows table consumer with resolution %s", resolution.Interval),
				c.migrationsStepCreateFlowsConsumerTable(resolution),
			}, {
				fmt.Sprintf("configure TTL for flows table with resolution %s", resolution.Interval),
				c.migrationsStepSetTTLFlowsTable(resolution),
			},
		}...)
	}
	steps = append(steps, []migrationStepWithDescription{
		{"create exporters view", c.migrationStepCreateExportersView},
		{"create raw flows table", c.migrationStepCreateRawFlowsTable},
		{"create raw flows consumer view", c.migrationStepCreateRawFlowsConsumerView},
		{"create raw flows errors view", c.migrationStepCreateRawFlowsErrorsView},
	}...)

	count := 0
	total := 0
	for _, step := range steps {
		total++
		l := c.r.Logger.With().Str("step", step.Description).Logger()
		l.Debug().Msg("checking migration step")
		step := step.Step(ctx, l, c.d.ClickHouse)
		rows, err := c.d.ClickHouse.Query(ctx, step.CheckQuery, step.Args...)
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
				c.metrics.migrationsNotApplied.Inc()
				continue
			} else {
				l.Debug().Msg("got 0, executing step")
			}
		} else {
			l.Debug().Msg("no result, executing step")
		}
		rows.Close()
		if err := step.Do(); err != nil {
			l.Err(err).Msg("cannot execute migration step")
			return fmt.Errorf("during migration step: %w", err)
		}
		l.Info().Msg("migration step executed successfully")
		c.metrics.migrationsApplied.Inc()
		count++
	}

	if count == 0 {
		c.r.Debug().Msg("no migration needed")
	} else {
		c.r.Info().Msg("migrations done")
	}
	close(c.migrationsDone)
	c.metrics.migrationsRunning.Set(0)
	c.metrics.migrationsVersion.Set(float64(total))

	// Reload dictionaries
	if err := c.d.ClickHouse.Exec(ctx, "SYSTEM RELOAD DICTIONARIES"); err != nil {
		c.r.Err(err).Msg("unable to reload dictionaries after migration")
	}

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
	_, port, err := net.SplitHostPort(c.d.HTTP.LocalAddr().String())
	if err != nil {
		return "", fmt.Errorf("cannot get HTTP port: %w", err)
	}
	base := fmt.Sprintf("http://%s",
		net.JoinHostPort(localAddr.IP.String(), port))
	c.r.Debug().Msgf("detected base URL is %s", base)
	return base, nil
}
