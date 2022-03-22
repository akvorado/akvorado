package flow

import (
	"bytes"
	"net"
	"time"

	"github.com/netsampler/goflow2/decoders/netflow"
	goflowmessage "github.com/netsampler/goflow2/pb"
	"github.com/netsampler/goflow2/producer"
)

// decodeFlow decodes the provided payload.
func (c *Component) decodeFlow(payload []byte, source *net.UDPAddr) {
	key := source.IP.String()

	c.templatesLock.RLock()
	templates, ok := c.templates[key]
	c.templatesLock.RUnlock()
	if !ok {
		templates = &templateSystem{
			c:         c,
			templates: netflow.CreateTemplateSystem(),
			key:       key,
		}
		c.templatesLock.Lock()
		c.templates[key] = templates
		c.templatesLock.Unlock()
	}
	c.samplingLock.RLock()
	sampling, ok := c.sampling[key]
	c.samplingLock.RUnlock()
	if !ok {
		sampling = producer.CreateSamplingSystem()
		c.samplingLock.Lock()
		c.sampling[key] = sampling
		c.samplingLock.Unlock()
	}

	timeTrackStart := time.Now()
	ts := uint64(timeTrackStart.UTC().Unix())
	buf := bytes.NewBuffer(payload)
	msgDec, err := netflow.DecodeMessage(buf, templates)

	if err != nil {
		switch err.(type) {
		case *netflow.ErrorTemplateNotFound:
			c.metrics.netflowErrors.WithLabelValues(key, "template_not_found").
				Inc()
		default:
			c.metrics.decoderErrors.WithLabelValues("netflow").
				Inc()
			c.metrics.netflowErrors.WithLabelValues(key, "error_decoding").
				Inc()
		}
		return
	}

	var flowMessageSet []*goflowmessage.FlowMessage

	switch msgDecConv := msgDec.(type) {
	case netflow.NFv9Packet:
		c.metrics.netflowStats.WithLabelValues(key, "9").
			Inc()

		for _, fs := range msgDecConv.FlowSets {
			switch fsConv := fs.(type) {
			case netflow.TemplateFlowSet:
				c.metrics.netflowSetStatsSum.WithLabelValues(key, "9", "TemplateFlowSet").
					Inc()
				c.metrics.netflowSetRecordsStatsSum.WithLabelValues(key, "9", "TemplateFlowSet").
					Add(float64(len(fsConv.Records)))
			case netflow.NFv9OptionsTemplateFlowSet:
				c.metrics.netflowSetStatsSum.WithLabelValues(key, "9", "OptionsTemplateFlowSet").
					Inc()
				c.metrics.netflowSetRecordsStatsSum.WithLabelValues(key, "9", "OptionsTemplateFlowSet").
					Add(float64(len(fsConv.Records)))
			case netflow.OptionsDataFlowSet:
				c.metrics.netflowSetStatsSum.WithLabelValues(key, "9", "OptionsDataFlowSet").
					Inc()
				c.metrics.netflowSetRecordsStatsSum.WithLabelValues(key, "9", "OptionsDataFlowSet").
					Add(float64(len(fsConv.Records)))
			case netflow.DataFlowSet:
				c.metrics.netflowSetStatsSum.WithLabelValues(key, "9", "DataFlowSet").
					Inc()
				c.metrics.netflowSetRecordsStatsSum.WithLabelValues(key, "9", "DataFlowSet").
					Add(float64(len(fsConv.Records)))
			}
		}
		flowMessageSet, err = producer.ProcessMessageNetFlow(msgDecConv, sampling)

		for _, fmsg := range flowMessageSet {
			fmsg.TimeReceived = ts
			fmsg.SamplerAddress = source.IP
			timeDiff := fmsg.TimeReceived - fmsg.TimeFlowEnd
			c.metrics.netflowTimeStatsSum.WithLabelValues(key, "9").
				Observe(float64(timeDiff))
		}
	default:
		c.metrics.netflowStats.WithLabelValues(key, "unknown").
			Inc()
		return
	}

	timeTrackStop := time.Now()
	c.metrics.decoderTime.WithLabelValues("netflow").
		Observe(float64((timeTrackStop.Sub(timeTrackStart)).Nanoseconds()) / 1000 / 1000 / 1000)
	c.metrics.decoderStats.WithLabelValues("netflow").
		Inc()

	for _, fmsg := range flowMessageSet {
		c.sendFlow(convert(fmsg))
	}
}

// convert a flow message from goflow2 to our own format. This is not
// the most efficient way.
func convert(input *goflowmessage.FlowMessage) *FlowMessage {
	return &FlowMessage{
		TimeReceived:     input.TimeReceived,
		SequenceNum:      input.SequenceNum,
		SamplingRate:     input.SamplingRate,
		FlowDirection:    input.FlowDirection,
		SamplerAddress:   net.IP(input.SamplerAddress).To16(),
		TimeFlowStart:    input.TimeFlowStart,
		TimeFlowEnd:      input.TimeFlowEnd,
		Bytes:            input.Bytes,
		Packets:          input.Packets,
		SrcAddr:          net.IP(input.SrcAddr).To16(),
		DstAddr:          net.IP(input.DstAddr).To16(),
		Etype:            input.Etype,
		Proto:            input.Proto,
		SrcPort:          input.SrcPort,
		DstPort:          input.DstPort,
		InIf:             input.InIf,
		OutIf:            input.OutIf,
		IPTos:            input.IPTos,
		ForwardingStatus: input.ForwardingStatus,
		IPTTL:            input.IPTTL,
		TCPFlags:         input.TCPFlags,
		IcmpType:         input.IcmpType,
		IcmpCode:         input.IcmpCode,
		IPv6FlowLabel:    input.IPv6FlowLabel,
		FragmentId:       input.FragmentId,
		FragmentOffset:   input.FragmentOffset,
		BiFlowDirection:  input.BiFlowDirection,
		SrcAS:            input.SrcAS,
		DstAS:            input.DstAS,
		SrcNet:           input.SrcNet,
		DstNet:           input.DstNet,
	}
}
