package flow

import (
	"akvorado/reporter"
)

type metrics struct {
	trafficBytes         *reporter.CounterVec
	trafficPackets       *reporter.CounterVec
	trafficPacketSizeSum *reporter.SummaryVec
	trafficErrors        *reporter.CounterVec
	trafficLoopTime      *reporter.SummaryVec

	decoderStats  *reporter.CounterVec
	decoderErrors *reporter.CounterVec
	decoderTime   *reporter.SummaryVec

	outgoingQueueFullTotal reporter.Counter
}

func (c *Component) initMetrics() {
	c.metrics.trafficBytes = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "traffic_bytes",
			Help: "Bytes received by the application.",
		},
		[]string{"sampler", "type"},
	)
	c.metrics.trafficPackets = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "traffic_packets",
			Help: "Packets received by the application.",
		},
		[]string{"sampler", "type"},
	)
	c.metrics.trafficPacketSizeSum = c.r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "traffic_summary_size_bytes",
			Help:       "Summary of packet size.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"sampler", "type"},
	)
	c.metrics.trafficErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "traffic_errors",
			Help: "Errors while receiving packets by the application.",
		},
		[]string{"type"},
	)
	c.metrics.trafficLoopTime = c.r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "traffic_loop_time_seconds",
			Help:       "How much time is spend in busy/idle state.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"worker", "state"},
	)

	c.metrics.decoderStats = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "decoder_count",
			Help: "Decoder processed count.",
		},
		[]string{"name"},
	)
	c.metrics.decoderErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "decoder_error_count",
			Help: "Decoder processed error count.",
		},
		[]string{"name"},
	)
	c.metrics.decoderTime = c.r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "summary_decoding_time_seconds",
			Help:       "Decoding time summary.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"name"},
	)

	c.metrics.outgoingQueueFullTotal = c.r.Counter(
		reporter.CounterOpts{
			Name: "outgoing_queue_full_total",
			Help: "Number of time the outgoing queue was full when sending a flow.",
		},
	)
}
