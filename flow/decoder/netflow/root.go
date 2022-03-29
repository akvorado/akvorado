// Package netflow handles NetFlow v9 and IPFIX decoding.
package netflow

import (
	"bytes"
	"strconv"
	"sync"

	"github.com/netsampler/goflow2/decoders/netflow"
	"github.com/netsampler/goflow2/producer"

	"akvorado/flow/decoder"
	"akvorado/reporter"
)

// Decoder contains the state for the Netflow v9 decoder.
type Decoder struct {
	r *reporter.Reporter

	// Templates and sampling
	templatesLock sync.RWMutex
	templates     map[string]*templateSystem
	samplingLock  sync.RWMutex
	sampling      map[string]producer.SamplingRateSystem

	metrics struct {
		errors             *reporter.CounterVec
		stats              *reporter.CounterVec
		setRecordsStatsSum *reporter.CounterVec
		setStatsSum        *reporter.CounterVec
		timeStatsSum       *reporter.SummaryVec
		templatesStats     *reporter.CounterVec
	}
}

// New instantiates a new netflow decoder.
func New(r *reporter.Reporter) decoder.Decoder {
	nd := &Decoder{
		r:         r,
		templates: map[string]*templateSystem{},
		sampling:  map[string]producer.SamplingRateSystem{},
	}

	nd.metrics.errors = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_count",
			Help: "Netflows processed errors.",
		},
		[]string{"exporter", "error"},
	)
	nd.metrics.stats = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "count",
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
	nd.metrics.timeStatsSum = nd.r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "delay_summary_seconds",
			Help:       "Netflows time difference between time of flow and processing.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"exporter", "version"},
	)
	nd.metrics.templatesStats = nd.r.CounterVec(
		reporter.CounterOpts{
			Name: "templates_count",
			Help: "Netflows Template count.",
		},
		[]string{"exporter", "version", "obs_domain_id", "template_id", "type"},
	)

	return nd
}

type templateSystem struct {
	nd        *Decoder
	key       string
	templates *netflow.BasicTemplateSystem
}

func (s *templateSystem) AddTemplate(version uint16, obsDomainID uint32, template interface{}) {
	s.templates.AddTemplate(version, obsDomainID, template)

	var (
		templateID uint16
		typeStr    string
	)
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
}

func (s *templateSystem) GetTemplate(version uint16, obsDomainID uint32, templateID uint16) (interface{}, error) {
	return s.templates.GetTemplate(version, obsDomainID, templateID)
}

// Decode decodes a Netflow payload.
func (nd *Decoder) Decode(in decoder.RawFlow) []*decoder.FlowMessage {
	key := in.Source.String()
	nd.templatesLock.RLock()
	templates, ok := nd.templates[key]
	nd.templatesLock.RUnlock()
	if !ok {
		templates = &templateSystem{
			nd:        nd,
			templates: netflow.CreateTemplateSystem(),
			key:       key,
		}
		nd.templatesLock.Lock()
		nd.templates[key] = templates
		nd.templatesLock.Unlock()
	}
	nd.samplingLock.RLock()
	sampling, ok := nd.sampling[key]
	nd.samplingLock.RUnlock()
	if !ok {
		sampling = producer.CreateSamplingSystem()
		nd.samplingLock.Lock()
		nd.sampling[key] = sampling
		nd.samplingLock.Unlock()
	}

	ts := uint64(in.TimeReceived.UTC().Unix())
	buf := bytes.NewBuffer(in.Payload)
	msgDec, err := netflow.DecodeMessage(buf, templates)

	if err != nil {
		switch err.(type) {
		case *netflow.ErrorTemplateNotFound:
			nd.metrics.errors.WithLabelValues(key, "template_not_found").Inc()
		default:
			nd.metrics.errors.WithLabelValues(key, "error_decoding").Inc()
		}
		return nil
	}

	var (
		version  string
		flowSets []interface{}
	)

	// Update some stats
	switch msgDecConv := msgDec.(type) {
	case netflow.IPFIXPacket:
		version = "10"
		flowSets = msgDecConv.FlowSets
	case netflow.NFv9Packet:
		version = "9"
		flowSets = msgDecConv.FlowSets
	default:
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

	flowMessageSet, err := producer.ProcessMessageNetFlow(msgDec, sampling)
	for _, fmsg := range flowMessageSet {
		fmsg.TimeReceived = ts
		fmsg.SamplerAddress = in.Source
		timeDiff := fmsg.TimeReceived - fmsg.TimeFlowEnd
		nd.metrics.timeStatsSum.WithLabelValues(key, version).
			Observe(float64(timeDiff))
	}

	results := make([]*decoder.FlowMessage, len(flowMessageSet))
	for idx, fmsg := range flowMessageSet {
		results[idx] = decoder.ConvertGoflowToFlowMessage(fmsg)
	}

	return results
}

// Name returns the name of the decoder.
func (nd *Decoder) Name() string {
	return "netflow"
}
