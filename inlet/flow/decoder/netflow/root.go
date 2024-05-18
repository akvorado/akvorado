// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package netflow handles NetFlow v9 and IPFIX decoding.
package netflow

import (
	"bytes"
	"errors"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/netsampler/goflow2/v2/decoders/netflow"

	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/flow/decoder"
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
		errors             *reporter.CounterVec
		stats              *reporter.CounterVec
		setRecordsStatsSum *reporter.CounterVec
		setStatsSum        *reporter.CounterVec
		templatesStats     *reporter.CounterVec
	}
	useTsFromNetflowsPacket bool
	useTsFromFirstSwitched  bool
}

// New instantiates a new netflow decoder.
func New(r *reporter.Reporter, dependencies decoder.Dependencies, option decoder.Option) decoder.Decoder {
	nd := &Decoder{
		r:                       r,
		d:                       dependencies,
		errLogger:               r.Sample(reporter.BurstSampler(30*time.Second, 3)),
		templates:               map[string]*templateSystem{},
		sampling:                map[string]*samplingRateSystem{},
		useTsFromNetflowsPacket: option.TimestampSource == decoder.TimestampSourceNetflowPacket,
		useTsFromFirstSwitched:  option.TimestampSource == decoder.TimestampSourceNetflowFirstSwitched,
	}

	nd.metrics.errors = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Netflows processed errors.",
		},
		[]string{"exporter", "error"},
	)
	nd.metrics.stats = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "flows_total",
			Help: "Netflows processed.",
		},
		[]string{"exporter", "version"},
	)
	nd.metrics.setRecordsStatsSum = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "flowset_records_sum",
			Help: "Netflows FlowSets sum of records.",
		},
		[]string{"exporter", "version", "type"},
	)
	nd.metrics.setStatsSum = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "flowset_sum",
			Help: "Netflows FlowSets sum.",
		},
		[]string{"exporter", "version", "type"},
	)
	nd.metrics.templatesStats = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "templates_total",
			Help: "Netflows Template count.",
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

	s.nd.metrics.templatesStats.WithLabelValues(
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
func (nd *Decoder) Decode(in decoder.RawFlow) []*schema.FlowMessage {
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

	var sysUptime uint64
	ts := uint64(in.TimeReceived.UTC().Unix())
	buf := bytes.NewBuffer(in.Payload)
	var (
		packetNFv9  netflow.NFv9Packet
		packetIPFIX netflow.IPFIXPacket
	)
	if err := netflow.DecodeMessageVersion(buf, templates, &packetNFv9, &packetIPFIX); err != nil {
		nd.metrics.errors.WithLabelValues(key, "NetFlow/IPFIX decoding error").Inc()
		if !errors.Is(err, netflow.ErrorTemplateNotFound) {
			nd.errLogger.Err(err).Str("exporter", key).Msg("error while decoding NetFlow/IPFIX")
		} else {
			nd.errLogger.Debug().Str("exporter", key).Msg("template not received yet")
		}
		return nil
	}

	var (
		version  string
		flowSets []interface{}
	)

	// Update some stats
	if packetNFv9.Version == 9 {
		version = "9"
		flowSets = packetNFv9.FlowSets
		if nd.useTsFromNetflowsPacket || nd.useTsFromFirstSwitched {
			ts = uint64(packetNFv9.UnixSeconds)
			sysUptime = uint64(packetNFv9.SystemUptime)
		}
	} else if packetIPFIX.Version == 10 {
		version = "10"
		flowSets = packetIPFIX.FlowSets
		if nd.useTsFromNetflowsPacket || nd.useTsFromFirstSwitched {
			ts = uint64(packetIPFIX.ExportTime)
			sysUptime = uint64(packetNFv9.SystemUptime)
		}
	} else {
		nd.metrics.stats.WithLabelValues(key, "unknown").
			Inc()
		return nil
	}
	nd.metrics.stats.WithLabelValues(key, version).Inc()
	for _, fs := range flowSets {
		switch fsConv := fs.(type) {
		case netflow.TemplateFlowSet:
			nd.metrics.setStatsSum.WithLabelValues(key, version, "TemplateFlowSet").
				Inc()
			nd.metrics.setRecordsStatsSum.WithLabelValues(key, version, "TemplateFlowSet").
				Add(float64(len(fsConv.Records)))
		case netflow.IPFIXOptionsTemplateFlowSet:
			nd.metrics.setStatsSum.WithLabelValues(key, version, "OptionsTemplateFlowSet").
				Inc()
			nd.metrics.setRecordsStatsSum.WithLabelValues(key, version, "OptionsTemplateFlowSet").
				Add(float64(len(fsConv.Records)))
		case netflow.NFv9OptionsTemplateFlowSet:
			nd.metrics.setStatsSum.WithLabelValues(key, version, "OptionsTemplateFlowSet").
				Inc()
			nd.metrics.setRecordsStatsSum.WithLabelValues(key, version, "OptionsTemplateFlowSet").
				Add(float64(len(fsConv.Records)))
		case netflow.OptionsDataFlowSet:
			nd.metrics.setStatsSum.WithLabelValues(key, version, "OptionsDataFlowSet").
				Inc()
			nd.metrics.setRecordsStatsSum.WithLabelValues(key, version, "OptionsDataFlowSet").
				Add(float64(len(fsConv.Records)))
		case netflow.DataFlowSet:
			nd.metrics.setStatsSum.WithLabelValues(key, version, "DataFlowSet").
				Inc()
			nd.metrics.setRecordsStatsSum.WithLabelValues(key, version, "DataFlowSet").
				Add(float64(len(fsConv.Records)))
		}
	}

	var flowMessageSet []*schema.FlowMessage
	var tsOffset uint64
	if nd.useTsFromFirstSwitched {
		tsOffset = ts - sysUptime
	}
	if packetNFv9.Version == 9 {
		flowMessageSet = nd.decodeNFv9(packetNFv9, sampling, tsOffset)
	} else if packetIPFIX.Version == 10 {
		flowMessageSet = nd.decodeIPFIX(packetIPFIX, sampling, tsOffset)
	}
	exporterAddress, _ := netip.AddrFromSlice(in.Source.To16())
	for _, fmsg := range flowMessageSet {
		if !nd.useTsFromFirstSwitched {
			fmsg.TimeReceived = ts
		}
		fmsg.ExporterAddress = exporterAddress
	}

	return flowMessageSet
}

// Name returns the name of the decoder.
func (nd *Decoder) Name() string {
	return "netflow"
}
