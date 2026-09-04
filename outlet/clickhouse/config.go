// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"time"
)

// ServerSelectionAlgorithm defines how a worker chooses a ClickHouse server for
// each batch.
type ServerSelectionAlgorithm int

const (
	// ServerSelectionStickyRandom pins each worker to a single, randomly-chosen
	// ClickHouse server for the lifetime of its connection. A new server is
	// picked (again at random) only when the connection breaks. This is the
	// default.
	ServerSelectionStickyRandom ServerSelectionAlgorithm = iota
	// ServerSelectionRoundRobin spreads a worker's batches across all configured
	// ClickHouse servers in round-robin order so insert load is balanced across
	// nodes.
	ServerSelectionRoundRobin
)

// Configuration describes the configuration for the ClickHouse exporter.
type Configuration struct {
	// GracePeriod defines how long to wait for flushing a batch to ClickHouse on shutdown.
	GracePeriod time.Duration `validate:"min=10s"`
	// MaximumBatchSize is the maximum number of rows to send to ClickHouse in one batch.
	MaximumBatchSize uint `validate:"min=1"`
	// MaximumWaitTime is the maximum number of seconds to wait before sending the current batch.
	MaximumWaitTime time.Duration `validate:"min=100ms"`
	// ServerSelection controls how each worker chooses a ClickHouse server for a
	// batch.
	ServerSelection ServerSelectionAlgorithm
	// minimumBatchSize the mininum number of rows before declaring underloaded and using async insert
	minimumBatchSize uint
}

const minimumBatchSizeDivider = 10

// DefaultConfiguration represents the default configuration for the ClickHouse exporter.
func DefaultConfiguration() Configuration {
	return Configuration{
		GracePeriod:      time.Minute,
		MaximumBatchSize: 50_000,
		MaximumWaitTime:  5 * time.Second,
		ServerSelection:  ServerSelectionStickyRandom,
	}
}
