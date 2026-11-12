// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package clickhouse

import (
	"context"
	"testing"

	"akvorado/common/schema"
)

// mockComponent is a mock version of the ClickHouse exporter.
type mockComponent struct {
	callback func(*schema.FlowMessage)
}

// NewMock creates a new mock exporter that calls the provided callback function with each received flow message.
func NewMock(_ *testing.T, callback func(*schema.FlowMessage)) Component {
	return &mockComponent{
		callback: callback,
	}
}

// NewWorker creates a new mock worker.
func (c *mockComponent) NewWorker(_ int, bf *schema.FlowMessage) Worker {
	return &mockWorker{
		c:  c,
		bf: bf,
	}
}

// mockWorker is a mock version of the ClickHouse worker.
type mockWorker struct {
	c  *mockComponent
	bf *schema.FlowMessage
}

// Finalize "sends" the current flow for testing purpose.
func (w *mockWorker) Finalize() {
	clone := *w.bf
	w.c.callback(&clone)
	w.bf.Clear() // Clear instead of finalizing
}

// MaybeFlush reports a flush: Finalize already "sent" the flows.
func (w *mockWorker) MaybeFlush(context.Context) (WorkerStatus, bool) {
	return WorkerStatusIdle, true
}

// Flush is a no-op: Finalize already "sent" the flows.
func (w *mockWorker) Flush(context.Context) error {
	return nil
}

// Close is a no-op for the mock worker.
func (w *mockWorker) Close() {}
