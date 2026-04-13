// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"akvorado/common/clickhousedb"
	"akvorado/common/schema"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// migrateDatabase execute database migration
func (c *Component) migrateDatabase() error {
	ctx := c.t.Context(nil)

	// Set orchestrator URL
	if c.config.OrchestratorURL == "" {
		baseURL, err := c.guessHTTPBaseURL("1.1.1.1")
		if err != nil {
			return err
		}
		c.config.OrchestratorURL = baseURL
	}

	// Grab some information about the database
	var version string
	row := c.d.ClickHouse.QueryRow(ctx, `SELECT version()`)
	if err := row.Scan(&version); err != nil {
		c.r.Err(err).Msg("unable to parse database settings")
		return fmt.Errorf("unable to parse database settings: %w", err)
	}
	if err := validateVersion(version); err != nil {
		return fmt.Errorf("incorrect ClickHouse version: %w", err)
	}

	if c.d.ClickHouse.ClusterName() != "" {
		var shardNum uint64
		row = c.d.ClickHouse.QueryRow(ctx,
			`SELECT countDistinct(shard_num) AS num FROM system.clusters WHERE cluster = $1`,
			c.d.ClickHouse.ClusterName(),
		)
		if err := row.Scan(&shardNum); err != nil {
			c.r.Err(err).Msg("unable to parse cluster settings")
			return fmt.Errorf("unable to parse cluster settings: %w", err)
		}
		if shardNum == 0 {
			return errors.New("cannot get the number of shards for the cluster")
		}
		c.shards = int(shardNum)
		ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
			"alter_sync": 2,
		}))
	}

	// Create dictionaries
	err := c.wrapMigrations(
		ctx,
		func(ctx context.Context) error {
			return c.createDictionary(ctx, schema.DictionaryASNs, "hashed",
				"`asn` UInt32 INJECTIVE, `name` String", "asn")
		}, func(ctx context.Context) error {
			return c.createDictionary(ctx, schema.DictionaryProtocols, "hashed",
				"`proto` UInt8 INJECTIVE, `name` String, `description` String", "proto")
		}, func(ctx context.Context) error {
			return c.createDictionary(ctx, schema.DictionaryICMP, "complex_key_hashed",
				"`proto` UInt8, `type` UInt8, `code` UInt8, `name` String", "proto, type, code")
		}, func(ctx context.Context) error {
			return c.createDictionary(ctx, schema.DictionaryNetworks, "ip_trie",
				"`network` String, `name` String, `role` String, `site` String, `region` String, `city` String, `state` String, `country` String, `tenant` String, `asn` UInt32",
				"network")
		}, func(ctx context.Context) error {
			return c.createDictionary(ctx, schema.DictionaryTCP, "hashed",
				"`port` UInt16 INJECTIVE, `name` String", "port")
		}, func(ctx context.Context) error {
			return c.createDictionary(ctx, schema.DictionaryUDP, "hashed",
				"`port` UInt16 INJECTIVE, `name` String", "port")
		})
	if err != nil {
		return err
	}

	// Prepare custom dictionary migrations
	var dictMigrations []func(context.Context) error
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
			schemaStr = append(schemaStr, fmt.Sprintf("`%s` %s DEFAULT %s",
				a.Name, a.Type, quoteString(defaultValue)))
		}
		dictMigrations = append(dictMigrations, func(ctx context.Context) error {
			return c.createDictionary(
				ctx,
				fmt.Sprintf("custom_dict_%s", k),
				v.Layout,
				strings.Join(schemaStr[:], ", "),
				strings.Join(keys[:], ", "))
		})
	}
	// Create custom dictionaries
	err = c.wrapMigrations(ctx, dictMigrations...)
	if err != nil {
		return err
	}

	// Create the various non-raw flow tables
	for _, resolution := range c.config.Resolutions {
		err := c.wrapMigrations(ctx,
			func(ctx context.Context) error {
				return c.createOrUpdateFlowsTable(ctx, resolution)
			}, func(ctx context.Context) error {
				if resolution.Interval == 0 {
					return c.createDistributedTable(ctx, "flows")
				}
				return c.createDistributedTable(ctx, fmt.Sprintf("flows_%s", resolution.Interval))
			}, func(ctx context.Context) error {
				return c.createFlowsConsumerView(ctx, resolution)
			})
		if err != nil {
			return err
		}
	}

	// Remaining tables
	err = c.wrapMigrations(ctx,
		c.createExportersTable,
		c.createExportersConsumerView,
		c.createRawFlowsTable,
		c.createRawFlowsConsumerView,
	)
	if err != nil {
		return err
	}

	close(c.migrationsDone)
	c.metrics.migrationsRunning.Set(0)
	c.r.Info().Msg("database migration done")

	// Reload dictionaries
	if err := c.d.ClickHouse.ExecOnCluster(ctx, "SYSTEM RELOAD DICTIONARIES"); err != nil {
		c.r.Err(err).Msg("unable to reload dictionaries after migration")
	}

	return nil
}

// guessHTTPBaseURL tries to guess the appropriate URL to access our
// HTTP daemon. It tries to get our IP address using an unconnected
// UDP socket.
func (c *Component) guessHTTPBaseURL(ip string) (string, error) {
	// Get IP address
	address := net.JoinHostPort(ip, "80")
	conn, err := net.Dial("udp", address)
	if err != nil {
		return "", fmt.Errorf("cannot get our IP address: %w", err)
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)

	// Get HTTP port
	_, httpPort, err := net.SplitHostPort(c.d.HTTP.LocalAddr().String())
	if err != nil {
		return "", fmt.Errorf("cannot get HTTP port: %w", err)
	}

	// Build final URL
	base := fmt.Sprintf("http://%s",
		net.JoinHostPort(localAddr.IP.String(), httpPort))
	c.r.Debug().Msgf("detected base URL is %s", base)
	return base, nil
}

// ReloadDictionary will reload the specified dictionnary.
func (c *Component) ReloadDictionary(ctx context.Context, dictName string) error {
	if c.d.ClickHouse != nil {
		return c.d.ClickHouse.ExecOnCluster(ctx, fmt.Sprintf("SYSTEM RELOAD DICTIONARY %s.%s",
			clickhousedb.QuoteIdentifier(c.d.ClickHouse.DatabaseName()),
			clickhousedb.QuoteIdentifier(dictName)))
	}
	return nil
}
