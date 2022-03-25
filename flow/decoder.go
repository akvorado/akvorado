package flow

import (
	"time"

	"akvorado/flow/decoder"
	"akvorado/flow/decoder/netflow"
	"akvorado/flow/input"
)

// Message describes a decoded flow message.
type Message = decoder.FlowMessage

// decodeWith decode a flow with the provided decoder
func (c *Component) decodeWith(d decoder.Decoder, in input.Flow) {
	timeTrackStart := time.Now()
	decoded := d.Decode(in)
	timeTrackStop := time.Now()

	if decoded == nil {
		c.metrics.decoderErrors.WithLabelValues(d.Name()).
			Inc()
		return
	}
	c.metrics.decoderTime.WithLabelValues(d.Name()).
		Observe(float64((timeTrackStop.Sub(timeTrackStart)).Nanoseconds()) / 1000 / 1000 / 1000)
	c.metrics.decoderStats.WithLabelValues(d.Name()).
		Inc()

	for _, f := range decoded {
		c.sendFlow(f)
	}
}

var decoders = map[string]decoder.NewDecoderFunc{
	"netflow": netflow.New,
}
