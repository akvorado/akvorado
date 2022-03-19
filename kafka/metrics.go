package kafka

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	gometrics "github.com/rcrowley/go-metrics"

	"akvorado/reporter"
)

type metrics struct {
	c *Component

	messagesSent *reporter.CounterVec
	bytesSent    *reporter.CounterVec
	errors       *reporter.CounterVec

	kafkaIncomingByteRate  *reporter.MetricDesc
	kafkaOutgoingByteRate  *reporter.MetricDesc
	kafkaRequestRate       *reporter.MetricDesc
	kafkaRequestSize       *reporter.MetricDesc
	kafkaRequestLatency    *reporter.MetricDesc
	kafkaResponseRate      *reporter.MetricDesc
	kafkaResponseSize      *reporter.MetricDesc
	kafkaRequestsInFlight  *reporter.MetricDesc
	kafkaBatchSize         *reporter.MetricDesc
	kafkaRecordSendRate    *reporter.MetricDesc
	kafkaRecordsPerRequest *reporter.MetricDesc
	kafkaCompressionRatio  *reporter.MetricDesc
}

func (c *Component) initMetrics() {
	c.metrics.c = c

	c.metrics.messagesSent = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "sent_messages_total",
			Help: "Number of messages sent from a given sampler.",
		},
		[]string{"sampler"},
	)
	c.metrics.bytesSent = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "sent_bytes_total",
			Help: "Number of bytes sent from a given sampler.",
		},
		[]string{"sampler"},
	)
	c.metrics.errors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Number of errors when sending.",
		},
		[]string{"error"},
	)

	c.metrics.kafkaIncomingByteRate = c.r.MetricDesc(
		"brokers_incoming_byte_rate",
		"Bytes/second read off a given broker.",
		[]string{"broker"})
	c.metrics.kafkaOutgoingByteRate = c.r.MetricDesc(
		"brokers_outgoing_byte_rate",
		"Bytes/second written off a given broker.",
		[]string{"broker"})
	c.metrics.kafkaRequestRate = c.r.MetricDesc(
		"brokers_request_rate",
		"Requests/second sent to a given broker.",
		[]string{"broker"})
	c.metrics.kafkaRequestSize = c.r.MetricDesc(
		"brokers_request_size",
		"Distribution of the request size in bytes for a given broker.",
		[]string{"broker"})
	c.metrics.kafkaRequestLatency = c.r.MetricDesc(
		"brokers_request_latency_seconds",
		"Distribution of the request latency in ms for a given broker.",
		[]string{"broker"})
	c.metrics.kafkaResponseRate = c.r.MetricDesc(
		"brokers_response_rate",
		"Responses/second received from a given broker.",
		[]string{"broker"})
	c.metrics.kafkaResponseSize = c.r.MetricDesc(
		"brokers_response_bytes",
		"Distribution of the response size in bytes for a given broker.",
		[]string{"broker"})
	c.metrics.kafkaRequestsInFlight = c.r.MetricDesc(
		"brokers_inflight_requests",
		"The current number of in-flight requests awaiting a response for a given broker.",
		[]string{"broker"})
	c.metrics.kafkaBatchSize = c.r.MetricDesc(
		"producer_batch_bytes",
		"Distribution of the number of bytes sent per partition per request.",
		nil)
	c.metrics.kafkaRecordSendRate = c.r.MetricDesc(
		"producer_record_send_rate",
		"Records/second sent.",
		nil)
	c.metrics.kafkaRecordsPerRequest = c.r.MetricDesc(
		"producer_records_per_request",
		"Distribution of the number of records sent per request.",
		nil)
	c.metrics.kafkaCompressionRatio = c.r.MetricDesc(
		"producer_compression_ratio",
		"Distribution of the compression ratio times 100 of record batches.",
		nil)

	c.r.MetricCollector(c.metrics)
}

// Describe collected metrics
func (m metrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- m.kafkaIncomingByteRate
	ch <- m.kafkaOutgoingByteRate
	ch <- m.kafkaRequestRate
	ch <- m.kafkaRequestSize
	ch <- m.kafkaRequestLatency
	ch <- m.kafkaResponseRate
	ch <- m.kafkaResponseSize
	ch <- m.kafkaRequestsInFlight
	ch <- m.kafkaBatchSize
	ch <- m.kafkaRecordSendRate
	ch <- m.kafkaRecordsPerRequest
	ch <- m.kafkaCompressionRatio
}

// Collect metrics
func (m metrics) Collect(ch chan<- prometheus.Metric) {
	m.c.kafkaConfig.MetricRegistry.Each(func(name string, gom interface{}) {
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
			snap := gom.(gometrics.Counter).Snapshot()
			ch <- prometheus.MustNewConstMetric(m.kafkaRequestsInFlight,
				prometheus.GaugeValue, float64(snap.Count()), broker)
			return
		}
		// Producer-related
		if name == "batch-size" {
			gomHistogram(ch, m.kafkaBatchSize, gom)
			return
		}
		if name == "record-send-rate" {
			gomMeter(ch, m.kafkaRecordSendRate, gom)
			return
		}
		if name == "records-per-request" {
			gomHistogram(ch, m.kafkaRecordsPerRequest, gom)
			return
		}
		if name == "compression-ratio" {
			gomHistogram(ch, m.kafkaCompressionRatio, gom)
			return
		}
	})
}

func metricBroker(name string, prefix string) string {
	prefix = prefix + "-for-broker-"
	if strings.HasPrefix(name, prefix) {
		return strings.TrimPrefix(name, prefix)
	}
	return ""
}

func gomMeter(ch chan<- prometheus.Metric, desc *reporter.MetricDesc, m interface{}, labels ...string) {
	snap := m.(gometrics.Meter).Snapshot()
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, snap.Rate1(), labels...)
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
