// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package kafka

import (
	"testing"
	"time"

	"github.com/Shopify/sarama"

	"akvorado/common/helpers"
)

// SetupKafkaBroker configures a client to use for testing.
func SetupKafkaBroker(t *testing.T) (sarama.Client, []string) {
	broker := helpers.CheckExternalService(t, "Kafka", []string{"kafka", "localhost"}, "9092")

	// Wait for broker to be ready
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V2_8_1_0
	saramaConfig.Net.DialTimeout = 1 * time.Second
	saramaConfig.Net.ReadTimeout = 1 * time.Second
	saramaConfig.Net.WriteTimeout = 1 * time.Second
	ready := false
	var (
		client sarama.Client
		err    error
	)
	for i := 0; i < 90; i++ {
		if client != nil {
			client.Close()
		}
		client, err = sarama.NewClient([]string{broker}, saramaConfig)
		if err != nil {
			continue
		}
		if err := client.RefreshMetadata(); err != nil {
			continue
		}
		brokers := client.Brokers()
		if len(brokers) == 0 {
			continue
		}
		if err := brokers[0].Open(client.Config()); err != nil {
			continue
		}
		if connected, err := brokers[0].Connected(); err != nil || !connected {
			brokers[0].Close()
			continue
		}
		brokers[0].Close()
		ready = true
	}
	if !ready {
		t.Fatalf("broker is not ready")
	}

	return client, []string{broker}
}
