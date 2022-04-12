package decoder

import (
	"net"
	"time"

	"akvorado/common/reporter"
)

// Decoder is the interface each decoder should implement.
type Decoder interface {
	// Decoder takes a raw flow and returns a
	// slice of flow messages. Returning nil means there was an
	// error during decoding.
	Decode(in RawFlow) []*FlowMessage

	// Name returns the decoder name
	Name() string
}

// RawFlow is an undecoded flow.
type RawFlow struct {
	TimeReceived time.Time
	Payload      []byte
	Source       net.IP
}

// NewDecoderFunc is the signature of a function to instantiate a decoder.
type NewDecoderFunc func(*reporter.Reporter) Decoder
