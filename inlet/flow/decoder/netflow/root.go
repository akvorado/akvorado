// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package netflow handles NetFlow v9 and IPFIX decoding.
package netflow

import (
	"bytes"
	"net/netip"
	"strconv"
	"sync"

	"github.com/netsampler/goflow2/v2/decoders/netflow"
	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"

	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/flow/decoder"
)

// Decoder contains the state for the Netflow v9 decoder.
type Decoder struct {
	r *reporter.Reporter
	d decoder.Dependencies

	// Templates and sampling systems
	systemsLock sync.RWMutex
	templates   map[string]*templateSystem
	sampling    map[string]protoproducer.SamplingRateSystem

	metrics struct {
		errors             *reporter.CounterVec
		stats              *reporter.CounterVec
		setRecordsStatsSum *reporter.CounterVec
		setStatsSum        *reporter.CounterVec
		templatesStats     *reporter.CounterVec
	}
}

// New instantiates a new netflow decoder.
func New(r *reporter.Reporter, dependencies decoder.Dependencies) decoder.Decoder {
	nd := &Decoder{
		r:         r,
		d:         dependencies,
		templates: map[string]*templateSystem{},
		sampling:  map[string]protoproducer.SamplingRateSystem{},
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
		sampling = protoproducer.CreateSamplingSystem()
		nd.systemsLock.Lock()
		nd.sampling[key] = sampling
		nd.systemsLock.Unlock()
	}

	ts := uint64(in.TimeReceived.UTC().Unix())
	buf := bytes.NewBuffer(in.Payload)
	var (
		packetNFv9  netflow.NFv9Packet
		packetIPFIX netflow.IPFIXPacket
	)
	if err := netflow.DecodeMessageVersion(buf, templates, &packetNFv9, &packetIPFIX); err != nil {
		switch err.(type) {
		case *netflow.DecoderError:
			nd.metrics.errors.WithLabelValues(key, err.Error()).Inc()
		default:
			nd.metrics.errors.WithLabelValues(key, "error decoding").Inc()
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
	} else if packetIPFIX.Version == 10 {
		version = "10"
		flowSets = packetIPFIX.FlowSets
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
	if packetNFv9.Version == 9 {
		flowMessageSet = nd.decodeNFv9(packetNFv9, sampling)
	} else if packetIPFIX.Version == 10 {
		flowMessageSet = nd.decodeIPFIX(packetIPFIX, sampling)
	}
	exporterAddress, _ := netip.AddrFromSlice(in.Source.To16())
	for _, fmsg := range flowMessageSet {
		fmsg.TimeReceived = ts
		fmsg.ExporterAddress = exporterAddress
	}

	return flowMessageSet
}

// Name returns the name of the decoder.
func (nd *Decoder) Name() string {
	return "netflow"
}
