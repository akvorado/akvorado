// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"fmt"
	"time"

	"akvorado/common/pb"
	"akvorado/common/reporter"
)

type metrics struct {
	messagesReceived *reporter.CounterVec
	fetchesReceived  *reporter.CounterVec
	bytesReceived    *reporter.CounterVec
	errorsReceived   *reporter.CounterVec
	workers          reporter.GaugeFunc
	workerIncrease   reporter.Counter
	workerDecrease   reporter.Counter
	consumerLag      reporter.GaugeFunc
}

func (c *realComponent) initMetrics() {
	c.metrics.messagesReceived = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "received_messages_total",
			Help: "Number of messages received for a given worker.",
		},
		[]string{"worker"},
	)
	c.metrics.fetchesReceived = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "received_fetches_total",
			Help: "Number of fetches received for a given worker.",
		},
		[]string{"worker"},
	)
	c.metrics.bytesReceived = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "received_bytes_total",
			Help: "Number of bytes received for a given worker.",
		},
		[]string{"worker"},
	)
	c.metrics.errorsReceived = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "received_errors_total",
			Help: "Number of errors while handling received messages for a given worker.",
		},
		[]string{"worker"},
	)
	c.metrics.workers = c.r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "workers",
			Help: "Number of running workers",
		},
		func() float64 {
			c.workerMu.Lock()
			defer c.workerMu.Unlock()
			return float64(len(c.workers))
		},
	)
	c.metrics.workerIncrease = c.r.Counter(
		reporter.CounterOpts{
			Name: "worker_increase_total",
			Help: "Number of times a new worker was spawned.",
		},
	)
	c.metrics.workerDecrease = c.r.Counter(
		reporter.CounterOpts{
			Name: "worker_decrease_total",
			Help: "Number of times a new worker was stopped.",
		},
	)
	c.metrics.consumerLag = c.r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "consumergroup_lag_messages",
			Help: "Current consumer lag across all partitions (or -1 on errors).",
		},
		func() float64 {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			c.kadmClientMu.Lock()
			defer c.kadmClientMu.Unlock()
			if c.kadmClient == nil {
				return -1
			}

			lag, err := c.computeLagMetric(ctx)
			if err != nil {
				c.r.Err(err).Msg("lag metric refresh failed")
				return -1
			}
			return lag
		},
	)
}

func (c *realComponent) computeLagMetric(ctx context.Context) (float64, error) {
	lag, err := c.kadmClient.Lag(ctx, c.config.ConsumerGroup)
	if err != nil {
		return -1, fmt.Errorf("unable to compute Kafka group lag: %w", err)
	}

	// The map entry should exist, but let's check anyway to be safe
	perGroupLag, ok := lag[c.config.ConsumerGroup]
	if !ok {
		return -1, fmt.Errorf("unable to find Kafka consumer group %q", c.config.ConsumerGroup)
	}
	if perGroupLag.FetchErr != nil {
		return -1, fmt.Errorf("unable to fetch Kafka consumer group offsets %q: %w", c.config.ConsumerGroup, perGroupLag.FetchErr)
	}
	if perGroupLag.DescribeErr != nil {
		return -1, fmt.Errorf("unable to describe Kafka consumer group %q: %w", c.config.ConsumerGroup, perGroupLag.DescribeErr)
	}

	// Retrieve only the current topic as there may be several
	topic := fmt.Sprintf("%s-v%d", c.config.Topic, pb.Version)
	perPartitionGroupLag, ok := perGroupLag.Lag[topic]
	if !ok {
		return -1, fmt.Errorf("unable to find Kafka consumer group lag for topic %q", topic)
	}

	// Finally, sum the lag across all partitions
	var lagTotal int64
	for _, partitionLag := range perPartitionGroupLag {
		// Skip possibly unassigned partitions in case of rebalancing
		if partitionLag.IsEmpty() {
			continue
		}

		if partitionLag.Err != nil {
			memberOrInstanceID := partitionLag.Member.MemberID
			if partitionLag.Member.InstanceID != nil {
				memberOrInstanceID = *partitionLag.Member.InstanceID
			}
			return -1, fmt.Errorf("unable to compute Kafka consumer lag because of a commit error on group %q, member %q, partition %q: %w", c.config.ConsumerGroup, memberOrInstanceID, partitionLag.Partition, partitionLag.Err)
		}
		lagTotal += partitionLag.Lag
	}

	return float64(lagTotal), nil
}
