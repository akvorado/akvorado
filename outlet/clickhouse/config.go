// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"time"
)

// Configuration describes the configuration for the ClickHouse exporter.
type Configuration struct {
	// GracePeriod defines how long to wait for flushing a batch to ClickHouse on shutdown.
	GracePeriod time.Duration `validate:"min=10s"`
	// MaximumBatchSize is the maximum number of rows to send to ClickHouse in one batch.
	MaximumBatchSize uint `validate:"min=1"`
	// MaximumWaitTime is the maximum number of seconds to wait before sending the current batch.
	MaximumWaitTime time.Duration `validate:"min=100ms"`
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
	}
}
