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
	defer close(c.migrationsOnce)

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

	// Create dictionaries
	err := c.wrapMigrations(
		func() error {
			return c.createDictionary(ctx, "asns", "hashed",
				"`asn` UInt32 INJECTIVE, `name` String", "asn")
		}, func() error {
			return c.createDictionary(ctx, "protocols", "hashed",
				"`proto` UInt8 INJECTIVE, `name` String, `description` String", "proto")
		}, func() error {
			return c.createDictionary(ctx, "networks", "ip_trie",
				"`network` String, `name` String, `role` String, `site` String, `region` String, `tenant` String",
				"network")
		})
	if err != nil {
		return err
	}

	// Create the various non-raw flow tables
	for _, resolution := range c.config.Resolutions {
		err := c.wrapMigrations(
			func() error {
				return c.createOrUpdateFlowsTable(ctx, resolution)
			}, func() error {
				return c.createFlowsConsumerView(ctx, resolution)
			})
		if err != nil {
			return err
		}
	}

	// Remaining tables
	err = c.wrapMigrations(
		func() error {
			return c.createExportersView(ctx)
		}, func() error {
			return c.createRawFlowsTable(ctx)
		}, func() error {
			return c.createRawFlowsConsumerView(ctx)
		}, func() error {
			return c.createRawFlowsErrorsView(ctx)
		}, func() error {
			return c.setTTLSystemLogsTables(ctx)
		},
	)
	if err != nil {
		return err
	}

	close(c.migrationsDone)
	c.metrics.migrationsRunning.Set(0)

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
