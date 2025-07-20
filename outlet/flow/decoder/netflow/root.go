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
	"sync"
	"time"

	"github.com/netsampler/goflow2/v2/decoders/netflow"
	"github.com/netsampler/goflow2/v2/decoders/netflowlegacy"

	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"
)

// Decoder contains the state for the Netflow v9 decoder.
type Decoder struct {
	r         *reporter.Reporter
	d         decoder.Dependencies
	errLogger reporter.Logger

	// Templates and sampling systems
	systemsLock sync.RWMutex
	templates   map[string]*templateSystem
	sampling    map[string]*samplingRateSystem

	metrics struct {
		errors    *reporter.CounterVec
		packets   *reporter.CounterVec
		records   *reporter.CounterVec
		sets      *reporter.CounterVec
		templates *reporter.CounterVec
	}
}

// New instantiates a new netflow decoder.
func New(r *reporter.Reporter, dependencies decoder.Dependencies) decoder.Decoder {
	nd := &Decoder{
		r:         r,
		d:         dependencies,
		errLogger: r.Sample(reporter.BurstSampler(30*time.Second, 3)),
		templates: map[string]*templateSystem{},
		sampling:  map[string]*samplingRateSystem{},
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

	return nd
}

type templateSystem struct {
	nd        *Decoder
	key       string
	templates netflow.NetFlowTemplateSystem
}

func (s *templateSystem) AddTemplate(version uint16, obsDomainID uint32, templateID uint16, template interface{}) error {
	if err := s.templates.AddTemplate(version, obsDomainID, templateID, template); err != nil {
		return nil
	}

	var typeStr string
	switch templateIDConv := template.(type) {
	case netflow.IPFIXOptionsTemplateRecord:
		templateID = templateIDConv.TemplateId
		typeStr = "options_template"
	case netflow.NFv9OptionsTemplateRecord:
		templateID = templateIDConv.TemplateId
		typeStr = "options_template"
	case netflow.TemplateRecord:
		templateID = templateIDConv.TemplateId
		typeStr = "template"
	}

	s.nd.metrics.templates.WithLabelValues(
		s.key,
		strconv.Itoa(int(version)),
		strconv.Itoa(int(obsDomainID)),
		strconv.Itoa(int(templateID)),
		typeStr,
	).Inc()
	return nil
}

func (s *templateSystem) GetTemplate(version uint16, obsDomainID uint32, templateID uint16) (interface{}, error) {
	return s.templates.GetTemplate(version, obsDomainID, templateID)
}

func (s *templateSystem) RemoveTemplate(version uint16, obsDomainID uint32, templateID uint16) (interface{}, error) {
	return s.templates.RemoveTemplate(version, obsDomainID, templateID)
}

type samplingRateKey struct {
	version     uint16
	obsDomainID uint32
	samplerID   uint64
}

type samplingRateSystem struct {
	lock  sync.RWMutex
	rates map[samplingRateKey]uint32
}

func (s *samplingRateSystem) GetSamplingRate(version uint16, obsDomainID uint32, samplerID uint64) uint32 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	rate, _ := s.rates[samplingRateKey{
		version:     version,
		obsDomainID: obsDomainID,
		samplerID:   samplerID,
	}]
	return rate
}

func (s *samplingRateSystem) SetSamplingRate(version uint16, obsDomainID uint32, samplerID uint64, samplingRate uint32) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.rates[samplingRateKey{
		version:     version,
		obsDomainID: obsDomainID,
		samplerID:   samplerID,
	}] = samplingRate
}

// Decode decodes a Netflow payload.
func (nd *Decoder) Decode(in decoder.RawFlow, options decoder.Option, bf *schema.FlowMessage, finalize decoder.FinalizeFlowFunc) (int, error) {
	if len(in.Payload) < 2 {
		return 0, errors.New("payload too small")
	}
	key := in.Source.String()
	nd.systemsLock.RLock()
	templates, tok := nd.templates[key]
	sampling, sok := nd.sampling[key]
	nd.systemsLock.RUnlock()
	if !tok {
		templates = &templateSystem{
			nd:        nd,
			templates: netflow.CreateTemplateSystem(),
			key:       key,
		}
		nd.systemsLock.Lock()
		nd.templates[key] = templates
		nd.systemsLock.Unlock()
	}
	if !sok {
		sampling = &samplingRateSystem{
			rates: map[samplingRateKey]uint32{},
		}
		nd.systemsLock.Lock()
		nd.sampling[key] = sampling
		nd.systemsLock.Unlock()
	}

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
		if err := netflow.DecodeMessageNetFlow(buf, templates, &packetNFv9); err != nil {
			if !errors.Is(err, netflow.ErrorTemplateNotFound) {
				nd.errLogger.Err(err).Str("exporter", key).Msg("error while decoding NetFlow v9")
				nd.metrics.errors.WithLabelValues(key, "NetFlow v9 decoding error").Inc()
				return 0, fmt.Errorf("NetFlow v9 decoding error: %w", err)
			}
			nd.errLogger.Debug().Str("exporter", key).Msg("template not received yet")
			return 0, nil
		}
		versionStr = "9"
		flowSets = packetNFv9.FlowSets
		obsDomainID = packetNFv9.SourceId
		if options.TimestampSource == pb.RawFlow_TS_NETFLOW_PACKET || options.TimestampSource == pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED {
			ts = uint64(packetNFv9.UnixSeconds)
			sysUptime = uint64(packetNFv9.SystemUptime)
		}
		nd.decodeNFv9IPFIX(version, obsDomainID, flowSets, sampling, ts, sysUptime, options, bf, finalize2)
	case 10:
		var packetIPFIX netflow.IPFIXPacket
		if err := netflow.DecodeMessageIPFIX(buf, templates, &packetIPFIX); err != nil {
			if !errors.Is(err, netflow.ErrorTemplateNotFound) {
				nd.errLogger.Err(err).Str("exporter", key).Msg("error while decoding IPFIX")
				nd.metrics.errors.WithLabelValues(key, "IPFIX decoding error").Inc()
				return 0, fmt.Errorf("NetFlow v9 decoding error: %w", err)
			}
			nd.errLogger.Debug().Str("exporter", key).Msg("template not received yet")
			return 0, nil
		}
		versionStr = "10"
		flowSets = packetIPFIX.FlowSets
		obsDomainID = packetIPFIX.ObservationDomainId
		if options.TimestampSource == pb.RawFlow_TS_NETFLOW_PACKET {
			ts = uint64(packetIPFIX.ExportTime)
		}
		nd.decodeNFv9IPFIX(version, obsDomainID, flowSets, sampling, ts, sysUptime, options, bf, finalize2)
	default:
		nd.errLogger.Warn().Str("exporter", key).Msg("unknown NetFlow version")
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
