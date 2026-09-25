// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package kafka

import (
	"context"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/reporter"

	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// SetupKafkaBroker configures a client to use for testing.
func SetupKafkaBroker(t *testing.T) (*kgo.Client, []string) {
	broker := helpers.CheckExternalService(t, "Kafka",
		[]string{"kafka:9092", "127.0.0.1:9092"})

	// Wait for broker to be ready
	r := reporter.NewMock(t)
	opts, err := NewConfig(r, Configuration{
		Brokers: []string{broker},
	})
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}

	// Add additional options for testing
	opts = append(opts,
		kgo.RequestTimeoutOverhead(1*time.Second),
		kgo.ProduceRequestTimeout(1*time.Second),
		kgo.ConnIdleTimeout(1*time.Second),
		kgo.MetadataMinAge(100*time.Millisecond),
	)

	ready := false
	var client *kgo.Client
	for i := 0; i < 90 && !ready; i++ {
		if client != nil {
			client.Close()
		}
		if client, err = kgo.NewClient(opts...); err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		if err := client.Ping(ctx); err != nil {
			cancel()
			continue
		}
		cancel()
		ready = true
	}
	if !ready {
		t.Fatalf("broker is not ready")
	}

	return client, []string{broker}
}

// InterceptMessages sets up a ControlKey to intercept all messages produced to a fake cluster
// and calls the callback function for each record received.
func InterceptMessages(t *testing.T, cluster *kfake.Cluster, callback func(*kgo.Record)) {
	t.Helper()

	// Use ControlKey to intercept ProduceRequest messages
	cluster.ControlKey(0, func(kreq kmsg.Request) (kmsg.Response, error, bool) {
		cluster.KeepControl()
		if req, ok := kreq.(*kmsg.ProduceRequest); ok {
			for _, topicData := range req.Topics {
				topic := topicData.Topic
				if info := cluster.TopicIDInfo(topicData.TopicID); info != nil {
					topic = info.Topic
				}
				for _, partitionData := range topicData.Partitions {
					if partitionData.Records != nil {
						var batch kmsg.RecordBatch
						if err := batch.ReadFrom(partitionData.Records); err != nil {
							t.Fatalf("batch.ReadFrom() error:\n%+v", err)
						}
						records, err := kfake.BatchRecords(batch)
						if err != nil {
							t.Fatalf("BatchRecords() error:\n%+v", err)
						}
						for _, rec := range records {
							callback(&kgo.Record{
								Topic:     topic,
								Partition: partitionData.Partition,
								Key:       rec.Key,
								Value:     rec.Value,
							})
						}
					}
				}
			}
		}

		// Don't modify the response, just let it pass through
		return nil, nil, false
	})
}

var _ kfake.Logger = &Logger{}
