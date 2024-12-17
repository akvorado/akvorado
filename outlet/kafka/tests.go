// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package kafka

import (
	"context"
	"testing"
)

type mockComponent struct {
	config   Configuration
	incoming chan []byte
}

// NewMock instantiates a fake Kafka consumer that will produce messages sent on
// the returned channel.
func NewMock(_ *testing.T, config Configuration) (Component, chan<- []byte) {
	c := mockComponent{
		config:   config,
		incoming: make(chan []byte),
	}
	return &c, c.incoming
}

// StartWorkers start a set of workers to produce received messages.
func (c *mockComponent) StartWorkers(workerBuilder WorkerBuilderFunc) error {
	for i := range c.config.Workers {
		callback, shutdown := workerBuilder(i)
		defer shutdown()
		go func() {
			for {
				message, ok := <-c.incoming
				if !ok {
					return
				}
				callback(context.Background(), message)
			}
		}()
	}
	return nil
}

// Stop stops the mock component.
func (c *mockComponent) Stop() error {
	close(c.incoming)
	return nil
}
