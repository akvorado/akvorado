// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhousedb

import (
	"time"

	"akvorado/common/helpers"
)

// Configuration defines how we connect to a ClickHouse database
type Configuration struct {
	// Servers define the list of clickhouse servers to connect to (with ports)
	Servers []string `validate:"min=1,dive,listen"`
	// Cluster defines the cluster to operate on. This should not change
	// anything from a client point of view, but this switch some mode of
	// operations.
	Cluster string
	// Database defines the database to use
	Database string `validate:"required"`
	// Username defines the username to use for authentication
	Username string `validate:"required"`
	// Password defines the password to use for authentication
	Password string
	// MaxOpenConns tells how many parallel connections to ClickHouse we want
	MaxOpenConns int `validate:"min=1"`
	// DialTimeout tells how much time to wait when connecting to ClickHouse
	DialTimeout time.Duration `validate:"min=100ms"`
	// TLS defines TLS connection parameters, if empty, plain TCP will be used.
	TLS helpers.TLSConfiguration
}

// DefaultConfiguration represents the default configuration for connecting to ClickHouse
func DefaultConfiguration() Configuration {
	return Configuration{
		Servers:      []string{"127.0.0.1:9000"},
		Database:     "default",
		Username:     "default",
		MaxOpenConns: 10,
		DialTimeout:  5 * time.Second,
		TLS: helpers.TLSConfiguration{
			Enable: false,
			Verify: true,
		},
	}
}
