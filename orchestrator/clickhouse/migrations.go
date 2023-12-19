// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"

	"akvorado/common/reporter"
	"akvorado/common/schema"
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

	// Grab some information about the database
	row := c.d.ClickHouse.QueryRow(ctx, `SELECT getSetting('max_threads'), version()`)
	if err := row.Err(); err != nil {
		c.r.Err(err).Msg("unable to query database")
		return fmt.Errorf("unable to query database: %w", err)
	}
	var threads uint8
	var version string
	if err := row.Scan(&threads, &version); err != nil {
		c.r.Err(err).Msg("unable to parse database settings")
		return fmt.Errorf("unable to parse database settings: %w", err)
	}
	if c.config.Kafka.Consumers > int(threads) {
		c.r.Warn().Msgf("too many consumers requested, capping to %d", threads)
		c.config.Kafka.Consumers = int(threads)
	}
	if err := validateVersion(version); err != nil {
		return fmt.Errorf("incorrect Clickhouse version: %w", err)
	}

	// Create dictionaries
	err := c.wrapMigrations(
		func() error {
			return c.createDictionary(ctx, schema.DictionaryASNs, "hashed",
				"`asn` UInt32 INJECTIVE, `name` String", "asn")
		}, func() error {
			return c.createDictionary(ctx, schema.DictionaryProtocols, "hashed",
				"`proto` UInt8 INJECTIVE, `name` String, `description` String", "proto")
		}, func() error {
			return c.createDictionary(ctx, schema.DictionaryICMP, "complex_key_hashed",
				"`proto` UInt8, `type` UInt8, `code` UInt8, `name` String", "proto, type, code")
		}, func() error {
			return c.createDictionary(ctx, schema.DictionaryNetworks, "ip_trie",
				"`network` String, `name` String, `role` String, `site` String, `region` String, `city` String, `state` String, `country` String, `tenant` String, `asn` UInt32",
				"network")
		})
	if err != nil {
		return err
	}

	// Prepare custom dictionary migrations
	var dictMigrations []func() error
	for k, v := range c.d.Schema.GetCustomDictConfig() {
		var schemaStr []string
		var keys []string
		for _, a := range v.Keys {
			// This is a key. We need it in the schema and in primary keys.
			schemaStr = append(schemaStr, fmt.Sprintf("`%s` %s", a.Name, a.Type))
			keys = append(keys, a.Name)
		}

		for _, a := range v.Attributes {
			defaultValue := "None"
			if a.Default != "" {
				defaultValue = a.Default
			}
			// This is only an attribute. We only need it in the schema
			schemaStr = append(schemaStr, fmt.Sprintf("`%s` %s DEFAULT '%s'", a.Name, a.Type, defaultValue))
		}
		dictMigrations = append(dictMigrations, func() error {
			return c.createDictionary(
				ctx,
				fmt.Sprintf("custom_dict_%s", k),
				v.Layout,
				strings.Join(schemaStr[:], ", "),
				strings.Join(keys[:], ", "))
		})
	}
	// Create custom dictionaries
	err = c.wrapMigrations(dictMigrations...)
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
		},
	)
	if err != nil {
		return err
	}

	close(c.migrationsDone)
	c.metrics.migrationsRunning.Set(0)
	c.r.Info().Msg("database migration done")

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

// ReloadDictionary will reload the specified dictionnary.
func (c *Component) ReloadDictionary(ctx context.Context, dictName string) error {
	return c.d.ClickHouse.Exec(ctx, fmt.Sprintf("SYSTEM RELOAD DICTIONARY %s", dictName))
}
