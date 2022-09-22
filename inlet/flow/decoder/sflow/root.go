// SPDX-FileCopyrightText: 2022 Tchadel Icard
// SPDX-License-Identifier: AGPL-3.0-only

// Package sflow handles sFlow v5 decoding.
package sflow

import (
	"bytes"
	"net"

	"github.com/netsampler/goflow2/decoders/sflow"
	"github.com/netsampler/goflow2/producer"

	"akvorado/common/reporter"
	"akvorado/inlet/flow/decoder"
)

// localInterface is used for InIf and OutIf when the traffic is
// locally originated or terminated. We need to translate it to 0.
const localInterface = 0x3FFFFFFF

// Decoder contains the state for the sFlow v5 decoder.
type Decoder struct {
	r *reporter.Reporter

	metrics struct {
		errors                *reporter.CounterVec
		stats                 *reporter.CounterVec
		sampleRecordsStatsSum *reporter.CounterVec
		sampleStatsSum        *reporter.CounterVec
	}
}

// New instantiates a new sFlow decoder.
func New(r *reporter.Reporter) decoder.Decoder {
	nd := &Decoder{
		r: r,
	}

	nd.metrics.errors = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_count",
			Help: "sFlows processed errors.",
		},
		[]string{"exporter", "error"},
	)
	nd.metrics.stats = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "count",
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
func (nd *Decoder) Decode(in decoder.RawFlow) []*decoder.FlowMessage {
	buf := bytes.NewBuffer(in.Payload)
	key := in.Source.String()

	ts := uint64(in.TimeReceived.UTC().Unix())
	msgDec, err := sflow.DecodeMessage(buf)

	if err != nil {
		switch err.(type) {
		case *sflow.ErrorVersion:
			nd.metrics.errors.WithLabelValues(key, "error version").Inc()
		case *sflow.ErrorIPVersion:
			nd.metrics.errors.WithLabelValues(key, "error ip version").Inc()
		case *sflow.ErrorDataFormat:
			nd.metrics.errors.WithLabelValues(key, "error data format").Inc()
		default:
			nd.metrics.errors.WithLabelValues(key, "error decoding").Inc()
		}
		return nil
	}

	// Update some stats
	msgDecConv, ok := msgDec.(sflow.Packet)
	if !ok {
		nd.metrics.stats.WithLabelValues(key, "unknown", "unknwon").Inc()
		return nil
	}
	agent := net.IP(msgDecConv.AgentIP).String()
	version := "5"
	samples := msgDecConv.Samples
	nd.metrics.stats.WithLabelValues(key, agent, version).Inc()
	for _, s := range samples {
		switch sConv := s.(type) {
		case sflow.FlowSample:
			nd.metrics.sampleStatsSum.WithLabelValues(key, agent, version, "FlowSample").
				Inc()
			nd.metrics.sampleRecordsStatsSum.WithLabelValues(key, agent, version, "FlowSample").
				Add(float64(len(sConv.Records)))
		case sflow.CounterSample:
			nd.metrics.sampleStatsSum.WithLabelValues(key, agent, version, "CounterSample").
				Inc()
			nd.metrics.sampleRecordsStatsSum.WithLabelValues(key, agent, version, "CounterSample").
				Add(float64(len(sConv.Records)))
		case sflow.ExpandedFlowSample:
			nd.metrics.sampleStatsSum.WithLabelValues(key, agent, version, "ExpandedFlowSample").
				Inc()
			nd.metrics.sampleRecordsStatsSum.WithLabelValues(key, agent, version, "ExpandedFlowSample").
				Add(float64(len(sConv.Records)))
		}
	}

	flowMessageSet, _ := producer.ProcessMessageSFlow(msgDec)
	for _, fmsg := range flowMessageSet {
		fmsg.TimeReceived = ts
		fmsg.TimeFlowStart = ts
		fmsg.TimeFlowEnd = ts
	}

	results := make([]*decoder.FlowMessage, len(flowMessageSet))
	for idx, fmsg := range flowMessageSet {
		if fmsg.InIf == localInterface {
			fmsg.InIf = 0
		}
		if fmsg.OutIf == localInterface {
			fmsg.OutIf = 0
		}
		results[idx] = decoder.ConvertGoflowToFlowMessage(fmsg)
	}

	return results
}

// Name returns the name of the decoder.
func (nd *Decoder) Name() string {
	return "sflow"
}
