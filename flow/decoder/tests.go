//go:build !release

package decoder

// DummyDecoder is a simple decoder producing flows from random data.
// The payload is copied in IfDescription
type DummyDecoder struct{}

// Decode returns uninteresting flow messages.
func (dc *DummyDecoder) Decode(in RawFlow) []*FlowMessage {
	return []*FlowMessage{
		{
			TimeReceived:    uint64(in.TimeReceived.UTC().Unix()),
			SamplerAddress:  in.Source.To16(),
			Bytes:           uint64(len(in.Payload)),
			Packets:         1,
			InIfDescription: string(in.Payload),
		},
	}
}

// Name returns the original name.
func (dc *DummyDecoder) Name() string {
	return "dummy"
}
