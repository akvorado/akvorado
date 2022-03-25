package decoder

import (
	"akvorado/flow/input"
	"akvorado/reporter"
)

// Decoder is the interface each decoder should implement.
type Decoder interface {
	// Decoder takes a raw flow and returns a
	// slice of flow messages. Returning nil means there was an
	// error during decoding.
	Decode(in input.Flow) []*FlowMessage

	// Name returns the decoder name
	Name() string
}

// NewDecoderFunc is the signature of a function to instantiate a decoder.
type NewDecoderFunc func(*reporter.Reporter) Decoder
