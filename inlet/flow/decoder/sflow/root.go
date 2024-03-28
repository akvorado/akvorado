// SPDX-FileCopyrightText: 2022 Tchadel Icard
// SPDX-License-Identifier: AGPL-3.0-only

// Package sflow handles sFlow v5 decoding.
package sflow

import (
	"bytes"
	"net"
	"time"

	"github.com/netsampler/goflow2/v2/decoders/sflow"

	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/flow/decoder"
)

const (
	// interfaceLocal is used for InIf and OutIf when the traffic is
	// locally originated or terminated. We need to translate it to 0.
	interfaceLocal = 0x3fffffff
	// interfaceOutMask is the mask to interpret output interface type
	interfaceOutMask = 0xc0000000
	// interfaceOutDiscard is used for OutIf when the traffic is discarded
	interfaceOutDiscard = 0x40000000
	// interfaceOutMultiple is used when there are multiple output interfaces
	interfaceOutMultiple = 0x80000000
)

// Decoder contains the state for the sFlow v5 decoder.
type Decoder struct {
	r         *reporter.Reporter
	d         decoder.Dependencies
	errLogger reporter.Logger

	metrics struct {
		errors                *reporter.CounterVec
		stats                 *reporter.CounterVec
		sampleRecordsStatsSum *reporter.CounterVec
		sampleStatsSum        *reporter.CounterVec
	}
}

// New instantiates a new sFlow decoder.
func New(r *reporter.Reporter, dependencies decoder.Dependencies, _ decoder.Option) decoder.Decoder {
	nd := &Decoder{
		r:         r,
		d:         dependencies,
		errLogger: r.Sample(reporter.BurstSampler(30*time.Second, 3)),
	}

	nd.metrics.errors = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "sFlows processed errors.",
		},
		[]string{"exporter", "error"},
	)
	nd.metrics.stats = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "flows_total",
			Help: "sFlows processed.",
		},
		[]string{"exporter", "agent", "version"},
	)
	nd.metrics.sampleRecordsStatsSum = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "sample_records_sum",
			Help: "sFlows samples sum of records.",
		},
		[]string{"exporter", "agent", "version", "type"},
	)
	nd.metrics.sampleStatsSum = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "sample_sum",
			Help: "sFlows samples sum.",
		},
		[]string{"exporter", "agent", "version", "type"},
	)

	return nd
}

// Decode decodes an sFlow payload.
func (nd *Decoder) Decode(in decoder.RawFlow) []*schema.FlowMessage {
	buf := bytes.NewBuffer(in.Payload)
	key := in.Source.String()

	ts := uint64(in.TimeReceived.UTC().Unix())
	var packet sflow.Packet
	if err := sflow.DecodeMessageVersion(buf, &packet); err != nil {
		nd.metrics.errors.WithLabelValues(key, "sFlow decoding error").Inc()
		nd.errLogger.Err(err).Str("exporter", key).Msg("error while decoding sFlow")
		return nil
	}

	// Update some stats
	agent := net.IP(packet.AgentIP).String()
	version := "5"
	samples := packet.Samples
	nd.metrics.stats.WithLabelValues(key, agent, version).Inc()
	for _, s := range samples {
		switch sConv := s.(type) {
		case sflow.FlowSample:
			nd.metrics.sampleStatsSum.WithLabelValues(key, agent, version, "FlowSample").
				Inc()
			nd.metrics.sampleRecordsStatsSum.WithLabelValues(key, agent, version, "FlowSample").
				Add(float64(len(sConv.Records)))
		case sflow.ExpandedFlowSample:
			nd.metrics.sampleStatsSum.WithLabelValues(key, agent, version, "ExpandedFlowSample").
				Inc()
			nd.metrics.sampleRecordsStatsSum.WithLabelValues(key, agent, version, "ExpandedFlowSample").
				Add(float64(len(sConv.Records)))
		case sflow.CounterSample:
			nd.metrics.sampleStatsSum.WithLabelValues(key, agent, version, "CounterSample").
				Inc()
			nd.metrics.sampleRecordsStatsSum.WithLabelValues(key, agent, version, "CounterSample").
				Add(float64(len(sConv.Records)))
		}
	}

	flowMessageSet := nd.decode(packet)
	for _, fmsg := range flowMessageSet {
		fmsg.TimeReceived = ts
	}

	return flowMessageSet
}

// Name returns the name of the decoder.
func (nd *Decoder) Name() string {
	return "sflow"
}
