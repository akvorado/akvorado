// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	gometrics "github.com/rcrowley/go-metrics"

	"akvorado/common/reporter"
)

// Metrics represent generic Kafka metrics. It includes broker, producer and consumer.
type Metrics struct {
	registry gometrics.Registry

	// Broker
	kafkaIncomingByteRate *reporter.MetricDesc
	kafkaOutgoingByteRate *reporter.MetricDesc
	kafkaRequestRate      *reporter.MetricDesc
	kafkaRequestSize      *reporter.MetricDesc
	kafkaRequestLatency   *reporter.MetricDesc
	kafkaResponseRate     *reporter.MetricDesc
	kafkaResponseSize     *reporter.MetricDesc
	kafkaRequestsInFlight *reporter.MetricDesc

	// Producer
	kafkaProducerBatchSize         *reporter.MetricDesc
	kafkaProducerRecordSendRate    *reporter.MetricDesc
	kafkaProducerRecordsPerRequest *reporter.MetricDesc
	kafkaProducerCompressionRatio  *reporter.MetricDesc

	// Consumer
	kafkaConsumerBatchSize         *reporter.MetricDesc
	kafkaConsumerFetchRate         *reporter.MetricDesc
	kafkaConsumerFetchResponseSize *reporter.MetricDesc
	kafkaConsumerGroupJoin         *reporter.MetricDesc
	kafkaConsumerGroupJoinFailed   *reporter.MetricDesc
	kafkaConsumerGroupSync         *reporter.MetricDesc
	kafkaConsumerGroupSyncFailed   *reporter.MetricDesc
}

// Init initialize the Kafka-related metrics.
func (m Metrics) Init(r *reporter.Reporter, registry gometrics.Registry) {
	m.registry = registry

	m.kafkaIncomingByteRate = r.MetricDesc2(
		"brokers_incoming_byte_rate",
		"Bytes/second read off a given broker.",
		[]string{"broker"})
	m.kafkaOutgoingByteRate = r.MetricDesc2(
		"brokers_outgoing_byte_rate",
		"Bytes/second written off a given broker.",
		[]string{"broker"})
	m.kafkaRequestRate = r.MetricDesc2(
		"brokers_request_rate",
		"Requests/second sent to a given broker.",
		[]string{"broker"})
	m.kafkaRequestSize = r.MetricDesc2(
		"brokers_request_size",
		"Distribution of the request size in bytes for a given broker.",
		[]string{"broker"})
	m.kafkaRequestLatency = r.MetricDesc2(
		"brokers_request_latency_ms",
		"Distribution of the request latency in ms for a given broker.",
		[]string{"broker"})
	m.kafkaResponseRate = r.MetricDesc2(
		"brokers_response_rate",
		"Responses/second received from a given broker.",
		[]string{"broker"})
	m.kafkaResponseSize = r.MetricDesc2(
		"brokers_response_bytes",
		"Distribution of the response size in bytes for a given broker.",
		[]string{"broker"})
	m.kafkaRequestsInFlight = r.MetricDesc2(
		"brokers_inflight_requests",
		"The current number of in-flight requests awaiting a response for a given broker.",
		[]string{"broker"})
	m.kafkaProducerBatchSize = r.MetricDesc2(
		"producer_batch_bytes",
		"Distribution of the number of bytes sent per partition per request.",
		nil)
	m.kafkaProducerRecordSendRate = r.MetricDesc2(
		"producer_record_send_rate",
		"Records/second sent.",
		nil)
	m.kafkaProducerRecordsPerRequest = r.MetricDesc2(
		"producer_records_per_request",
		"Distribution of the number of records sent per request.",
		nil)
	m.kafkaProducerCompressionRatio = r.MetricDesc2(
		"producer_compression_ratio",
		"Distribution of the compression ratio times 100 of record batches.",
		nil)
	m.kafkaConsumerBatchSize = r.MetricDesc2(
		"consumer_batch_messages",
		"Distribution of the number of messages per batch.",
		nil,
	)
	m.kafkaConsumerFetchRate = r.MetricDesc2(
		"consumer_fetch_rate",
		"Fetch requests/second sent to all brokers.",
		nil,
	)
	m.kafkaConsumerFetchResponseSize = r.MetricDesc2(
		"consumer_fetch_bytes",
		"Distribution of the fetch response size in bytes.",
		nil,
	)
	m.kafkaConsumerGroupJoin = r.MetricDesc2(
		"consumer_group_join_total",
		"Total count of consumer group join attempts",
		[]string{"group"})
	m.kafkaConsumerGroupJoinFailed = r.MetricDesc2(
		"consumer_group_join_failed_total",
		"Total count of consumer group join failures.",
		[]string{"group"})
	m.kafkaConsumerGroupSync = r.MetricDesc2(
		"consumer_group_sync_total",
		"Total count of consumer group sync attempts",
		[]string{"group"})
	m.kafkaConsumerGroupSyncFailed = r.MetricDesc2(
		"consumer_group_sync_failed_total",
		"Total count of consumer group sync failures.",
		[]string{"group"})

	r.MetricCollector(m)
}

// Describe collected metrics
func (m Metrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- m.kafkaIncomingByteRate
	ch <- m.kafkaOutgoingByteRate
	ch <- m.kafkaRequestRate
	ch <- m.kafkaRequestSize
	ch <- m.kafkaRequestLatency
	ch <- m.kafkaResponseRate
	ch <- m.kafkaResponseSize
	ch <- m.kafkaRequestsInFlight
	ch <- m.kafkaProducerBatchSize
	ch <- m.kafkaProducerRecordSendRate
	ch <- m.kafkaProducerRecordsPerRequest
	ch <- m.kafkaProducerCompressionRatio
	ch <- m.kafkaConsumerBatchSize
	ch <- m.kafkaConsumerFetchRate
	ch <- m.kafkaConsumerFetchResponseSize
	ch <- m.kafkaConsumerGroupJoin
	ch <- m.kafkaConsumerGroupJoinFailed
	ch <- m.kafkaConsumerGroupSync
	ch <- m.kafkaConsumerGroupSyncFailed
}

// Collect metrics
func (m Metrics) Collect(ch chan<- prometheus.Metric) {
	m.registry.Each(func(name string, gom interface{}) {
		// Broker-related
		if broker := metricBroker(name, "incoming-byte-rate"); broker != "" {
			gomMeter(ch, m.kafkaIncomingByteRate, gom, broker)
			return
		}
		if broker := metricBroker(name, "outgoing-byte-rate"); broker != "" {
			gomMeter(ch, m.kafkaOutgoingByteRate, gom, broker)
			return
		}
		if broker := metricBroker(name, "request-rate"); broker != "" {
			gomMeter(ch, m.kafkaRequestRate, gom, broker)
			return
		}
		if broker := metricBroker(name, "request-size"); broker != "" {
			gomHistogram(ch, m.kafkaRequestSize, gom, broker)
			return
		}
		if broker := metricBroker(name, "request-latency-in-ms"); broker != "" {
			gomHistogram(ch, m.kafkaRequestLatency, gom, broker)
			return
		}
		if broker := metricBroker(name, "response-rate"); broker != "" {
			gomMeter(ch, m.kafkaResponseRate, gom, broker)
			return
		}
		if broker := metricBroker(name, "response-size"); broker != "" {
			gomHistogram(ch, m.kafkaResponseSize, gom, broker)
			return
		}
		if broker := metricBroker(name, "requests-in-flight"); broker != "" {
			gomCounter(ch, m.kafkaRequestsInFlight, gom, broker)
			return
		}
		// Producer-related
		if name == "batch-size" {
			gomHistogram(ch, m.kafkaProducerBatchSize, gom)
			return
		}
		if name == "record-send-rate" {
			gomMeter(ch, m.kafkaProducerRecordSendRate, gom)
			return
		}
		if name == "records-per-request" {
			gomHistogram(ch, m.kafkaProducerRecordsPerRequest, gom)
			return
		}
		if name == "compression-ratio" {
			gomHistogram(ch, m.kafkaProducerCompressionRatio, gom)
			return
		}
		// Consumer-related
		if name == "consumer-batch-size" {
			gomHistogram(ch, m.kafkaConsumerBatchSize, gom)
			return
		}
		if name == "consumer-fetch-rate" {
			gomMeter(ch, m.kafkaConsumerFetchRate, gom)
			return
		}
		if name == "consumer-fetch-response-size" {
			gomHistogram(ch, m.kafkaConsumerFetchResponseSize, gom)
			return
		}
		if groupID := metricGroupID(name, "consumer-group-join-total"); groupID != "" {
			gomCounter(ch, m.kafkaConsumerGroupJoin, gom, groupID)
			return
		}
		if groupID := metricGroupID(name, "consumer-group-join-failed"); groupID != "" {
			gomCounter(ch, m.kafkaConsumerGroupJoinFailed, gom, groupID)
			return
		}
		if groupID := metricGroupID(name, "consumer-group-sync-total"); groupID != "" {
			gomCounter(ch, m.kafkaConsumerGroupSync, gom, groupID)
			return
		}
		if groupID := metricGroupID(name, "consumer-group-sync-failed"); groupID != "" {
			gomCounter(ch, m.kafkaConsumerGroupSyncFailed, gom, groupID)
			return
		}
	})
}

func metricBroker(name, prefix string) string {
	prefix = prefix + "-for-broker-"
	if strings.HasPrefix(name, prefix) {
		return strings.TrimPrefix(name, prefix)
	}
	return ""
}

func metricGroupID(name, prefix string) string {
	prefix = prefix + "-"
	if strings.HasPrefix(name, prefix) {
		return strings.TrimPrefix(name, prefix)
	}
	return ""
}

func gomMeter(ch chan<- prometheus.Metric, desc *reporter.MetricDesc, m interface{}, labels ...string) {
	snap := m.(gometrics.Meter).Snapshot()
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, snap.Rate1(), labels...)
}

func gomCounter(ch chan<- prometheus.Metric, desc *reporter.MetricDesc, m interface{}, labels ...string) {
	snap := m.(gometrics.Counter).Snapshot()
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(snap.Count()), labels...)
}

func gomHistogram(ch chan<- prometheus.Metric, desc *reporter.MetricDesc, m interface{}, labels ...string) {
	snap := m.(gometrics.Histogram).Snapshot()
	buckets := map[float64]uint64{
		0.5:  uint64(snap.Percentile(0.5)),
		0.9:  uint64(snap.Percentile(0.9)),
		0.99: uint64(snap.Percentile(0.99)),
	}
	ch <- prometheus.MustNewConstHistogram(desc, uint64(snap.Count()), float64(snap.Sum()), buckets, labels...)
}
