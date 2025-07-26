// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-FileCopyrightText: 2020 Travis Bischel
// SPDX-License-Identifier: AGPL-3.0-only AND BSD-3-Clause

//go:build !release

package kafka

import (
	"context"
	"encoding/binary"
	"fmt"
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
		TLS: helpers.TLSConfiguration{
			Enable: false,
			Verify: true,
		},
	})
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}

	// Add additional options for testing
	opts = append(opts,
		kgo.RequestTimeoutOverhead(1*time.Second),
		kgo.ProduceRequestTimeout(1*time.Second),
		kgo.ConnIdleTimeout(1*time.Second),
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

// forEachBatchRecord iterates through all records in a record batch. This
// function is stolen from franz-go/pkg/kfake/data.go.
func forEachBatchRecord(batch kmsg.RecordBatch, cb func(kmsg.Record) error) error {
	records, err := kgo.DefaultDecompressor().Decompress(
		batch.Records,
		kgo.CompressionCodecType(batch.Attributes&0x0007),
	)
	if err != nil {
		return err
	}
	for range batch.NumRecords {
		rec := kmsg.NewRecord()
		err := rec.ReadFrom(records)
		if err != nil {
			return fmt.Errorf("corrupt batch: %w", err)
		}
		if err := cb(rec); err != nil {
			return err
		}
		length, amt := binary.Varint(records)
		records = records[length+int64(amt):]
	}
	if len(records) > 0 {
		return fmt.Errorf("corrupt batch, extra left over bytes after parsing batch: %v", len(records))
	}
	return nil
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
				for _, partitionData := range topicData.Partitions {
					if partitionData.Records != nil {
						var batch kmsg.RecordBatch
						if err := batch.ReadFrom(partitionData.Records); err != nil {
							t.Fatalf("batch.ReadFrom() error:\n%+v", err)
						}
						if err := forEachBatchRecord(batch, func(rec kmsg.Record) error {
							kgoRecord := &kgo.Record{
								Topic:     topicData.Topic,
								Partition: partitionData.Partition,
								Key:       rec.Key,
								Value:     rec.Value,
							}
							callback(kgoRecord)
							return nil
						}); err != nil {
							t.Fatalf("forEachBatchRecord() error:\n%+v", err)
						}
					}
				}
			}
		}

		// Don't modify the response, just let it pass through
		return nil, nil, false
	})
}
