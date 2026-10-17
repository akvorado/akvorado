// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package netflow handles NetFlow v9 and IPFIX decoding.
package netflow

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/netsampler/goflow2/v3/decoders/netflow"
	"github.com/netsampler/goflow2/v3/decoders/netflowlegacy"

	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"
)

// Decoder contains the state for the NetFlow v9 decoder.
type Decoder struct {
	r         *reporter.Reporter
	d         decoder.Dependencies
	errLogger reporter.Logger

	// Templates and sampling systems
	collection templateAndOptionCollection
	sequences  sequenceTracker

	metrics struct {
		errors    *reporter.CounterVec
		packets   *reporter.CounterVec
		records   *reporter.CounterVec
		sets      *reporter.CounterVec
		templates *reporter.CounterVec

		sequenceMissing   *reporter.CounterVec
		sequenceReordered *reporter.CounterVec
	}
}

// New instantiates a new netflow decoder.
func New(r *reporter.Reporter, dependencies decoder.Dependencies) decoder.Decoder {
	nd := &Decoder{
		r:         r,
		d:         dependencies,
		errLogger: r.Sample(reporter.BurstSampler(30*time.Second, 3)),
	}
	nd.collection = templateAndOptionCollection{
		nd:         nd,
		Collection: make(map[string]*templatesAndOptions),
	}

	nd.metrics.errors = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Number of NetFlow errors processed.",
		},
		[]string{"exporter", "error"},
	)
	nd.metrics.packets = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "packets_total",
			Help: "Number of NetFlow packets received.",
		},
		[]string{"exporter", "version"},
	)
	nd.metrics.sets = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "sets_total",
			Help: "Number of NetFlow flowsets received.",
		},
		[]string{"exporter", "version", "type"},
	)
	nd.metrics.records = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "records_total",
			Help: "Number of NetFlow records received.",
		},
		[]string{"exporter", "version", "type"},
	)
	nd.metrics.templates = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "templates_total",
			Help: "Number of NetFlow templates received.",
		},
		[]string{"exporter", "version", "obs_domain_id", "template_id", "type"},
	)
	nd.metrics.sequenceMissing = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "sequence_missing_total",
			Help: "Number of NetFlow v9 packets or IPFIX data records missing according to sequence numbers. Exact only with the by-exporter load-balancing of the inlet.",
		},
		[]string{"exporter", "version"},
	)
	nd.metrics.sequenceReordered = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "sequence_reordered_total",
			Help: "Number of NetFlow v9 or IPFIX packets with a sequence number lower than expected (reordered packet or exporter restart).",
		},
		[]string{"exporter", "version"},
	)

	return nd
}

// Decode decodes a NetFlow payload.
func (nd *Decoder) Decode(in decoder.RawFlow, options decoder.Options, bf *schema.FlowMessage, finalize decoder.FinalizeFlowFunc) (int, error) {
	if len(in.Payload) < 2 {
		return 0, errors.New("payload too small")
	}
	key := in.Source.String()
	tao := nd.collection.Get(key)

	var (
		sysUptime   uint64
		versionStr  string
		flowSets    []any
		obsDomainID uint32
	)
	version := binary.BigEndian.Uint16(in.Payload[:2])
	buf := bytes.NewBuffer(in.Payload[2:])
	ts := uint64(in.TimeReceived.UTC().Unix()) // may be altered later
	finalize2 := func() {
		if bf.TimeReceived == 0 {
			bf.TimeReceived = uint32(ts)
		}
		bf.ExporterAddress = in.Source
		finalize()
	}

	switch version {
	case 5:
		if options.DecapsulationProtocol != pb.RawFlow_DECAP_NONE {
			nd.metrics.errors.WithLabelValues(key, "non-encapsulated packet").Inc()
			return 0, nil
		}
		var packetNFv5 netflowlegacy.PacketNetFlowV5
		if err := netflowlegacy.DecodeMessage(buf, &packetNFv5); err != nil {
			nd.metrics.errors.WithLabelValues(key, "NetFlow v5 decoding error").Inc()
			nd.errLogger.Err(err).Str("exporter", key).Msg("error while decoding NetFlow v5")
			return 0, fmt.Errorf("NetFlow v5 decoding error: %w", err)
		}
		versionStr = "5"
		nd.metrics.sets.WithLabelValues(key, versionStr, "PDU").Inc()
		nd.metrics.records.WithLabelValues(key, versionStr, "PDU").
			Add(float64(len(packetNFv5.Records)))
		if options.TimestampSource == pb.RawFlow_TS_NETFLOW_PACKET || options.TimestampSource == pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED {
			ts = uint64(packetNFv5.UnixSecs)
			sysUptime = uint64(packetNFv5.SysUptime)
		}
		nd.decodeNFv5(&packetNFv5, ts, sysUptime, options, bf, finalize2)
	case 9:
		var packetNFv9 netflow.NFv9Packet
		if err := netflow.DecodeMessageNetFlow(buf, tao, netflow.FlowContext{}, &packetNFv9); err != nil {
			if !errors.Is(err, netflow.ErrorTemplateNotFound) {
				nd.errLogger.Err(err).Str("exporter", key).Msg("error while decoding NetFlow v9")
				nd.metrics.errors.WithLabelValues(key, "NetFlow v9 decoding error").Inc()
				return 0, fmt.Errorf("NetFlow v9 decoding error: %w", err)
			}
			// Other sets of the packet may still be decoded.
			nd.errLogger.Debug().Str("exporter", key).Msg("template not received yet")
		}
		versionStr = "9"
		flowSets = packetNFv9.FlowSets
		obsDomainID = packetNFv9.SourceId
		nd.observeSequence(key, version, obsDomainID, packetNFv9.SequenceNumber, 1, true)
		if options.TimestampSource == pb.RawFlow_TS_NETFLOW_PACKET || options.TimestampSource == pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED {
			ts = uint64(packetNFv9.UnixSeconds)
			sysUptime = uint64(packetNFv9.SystemUptime)
		}
		nd.decodeNFv9IPFIX(version, obsDomainID, flowSets, tao, ts, sysUptime, options, key, bf, finalize2)
	case 10:
		var packetIPFIX netflow.IPFIXPacket
		err := netflow.DecodeMessageIPFIX(buf, tao, netflow.FlowContext{}, &packetIPFIX)
		if err != nil {
			if !errors.Is(err, netflow.ErrorTemplateNotFound) {
				nd.errLogger.Err(err).Str("exporter", key).Msg("error while decoding IPFIX")
				nd.metrics.errors.WithLabelValues(key, "IPFIX decoding error").Inc()
				return 0, fmt.Errorf("IPFIX decoding error: %w", err)
			}
			// Other sets of the packet may still be decoded.
			nd.errLogger.Debug().Str("exporter", key).Msg("template not received yet")
		}
		versionStr = "10"
		flowSets = packetIPFIX.FlowSets
		obsDomainID = packetIPFIX.ObservationDomainId
		// Without all the templates, the number of data records is unknown.
		nd.observeSequence(key, version, obsDomainID, packetIPFIX.SequenceNumber,
			ipfixDataRecords(flowSets), err == nil)
		if options.TimestampSource == pb.RawFlow_TS_NETFLOW_PACKET {
			ts = uint64(packetIPFIX.ExportTime)
		}
		nd.decodeNFv9IPFIX(version, obsDomainID, flowSets, tao, ts, sysUptime, options, key, bf, finalize2)
	default:
		nd.errLogger.Warn().Str("exporter", key).Msgf("unknown NetFlow version %d", version)
		nd.metrics.packets.WithLabelValues(key, "unknown").
			Inc()
		return 0, errors.New("unkown NetFlow version")
	}
	nd.metrics.packets.WithLabelValues(key, versionStr).Inc()

	nb := 0
	for _, fs := range flowSets {
		switch fsConv := fs.(type) {
		case netflow.TemplateFlowSet:
			nd.metrics.sets.WithLabelValues(key, versionStr, "TemplateFlowSet").
				Inc()
			nd.metrics.records.WithLabelValues(key, versionStr, "TemplateFlowSet").
				Add(float64(len(fsConv.Records)))
		case netflow.IPFIXOptionsTemplateFlowSet:
			nd.metrics.sets.WithLabelValues(key, versionStr, "OptionsTemplateFlowSet").
				Inc()
			nd.metrics.records.WithLabelValues(key, versionStr, "OptionsTemplateFlowSet").
				Add(float64(len(fsConv.Records)))
		case netflow.NFv9OptionsTemplateFlowSet:
			nd.metrics.sets.WithLabelValues(key, versionStr, "OptionsTemplateFlowSet").
				Inc()
			nd.metrics.records.WithLabelValues(key, versionStr, "OptionsTemplateFlowSet").
				Add(float64(len(fsConv.Records)))
		case netflow.OptionsDataFlowSet:
			nd.metrics.sets.WithLabelValues(key, versionStr, "OptionsDataFlowSet").
				Inc()
			nd.metrics.records.WithLabelValues(key, versionStr, "OptionsDataFlowSet").
				Add(float64(len(fsConv.Records)))
		case netflow.DataFlowSet:
			nd.metrics.sets.WithLabelValues(key, versionStr, "DataFlowSet").
				Inc()
			nd.metrics.records.WithLabelValues(key, versionStr, "DataFlowSet").
				Add(float64(len(fsConv.Records)))
			nb += len(fsConv.Records)
		}
	}

	return nb, nil
}

// Name returns the name of the decoder.
func (nd *Decoder) Name() string {
	return "netflow"
}

// observeSequence updates the sequence state of an observation domain and the
// associated metrics.
func (nd *Decoder) observeSequence(exporter string, version uint16, obsDomainID, seq, increment uint32, incrementKnown bool) {
	missing, reordered := nd.sequences.observe(sequenceKey{
		exporter:    exporter,
		version:     version,
		obsDomainID: obsDomainID,
	}, seq, increment, incrementKnown)
	versionStr := strconv.Itoa(int(version))
	if missing > 0 {
		nd.metrics.sequenceMissing.WithLabelValues(exporter, versionStr).Add(float64(missing))
	}
	if reordered {
		nd.metrics.sequenceReordered.WithLabelValues(exporter, versionStr).Inc()
	}
}

// ipfixDataRecords returns the number of data records of an IPFIX message,
// including the ones described by options templates.
func ipfixDataRecords(flowSets []any) uint32 {
	var count uint32
	for _, fs := range flowSets {
		switch fsConv := fs.(type) {
		case netflow.DataFlowSet:
			count += uint32(len(fsConv.Records))
		case netflow.OptionsDataFlowSet:
			count += uint32(len(fsConv.Records))
		}
	}
	return count
}
