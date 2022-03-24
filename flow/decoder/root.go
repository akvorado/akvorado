package decoder

import (
	"net"

	"akvorado/reporter"
)

// Decoder is the interface each decoder should implement.
type Decoder interface {
	// Decoder takes a payload, an origin IP address and returns a
	// slice of flow messages. Returning nil means there was an
	// error during decoding.
	Decode(payload []byte, source net.IP) []*FlowMessage

	// Name returns the decoder name
	Name() string
}

// NewDecoderFunc is the signature of a function to instantiate a decoder.
type NewDecoderFunc func(*reporter.Reporter) Decoder

// Register allows a decoder to register itself to the registry
// of decoders. The registration should happen during initialization.
func Register(name string, init NewDecoderFunc) {
	registeredDecoders[name] = init
}

// New returns a new instance of a decoder. Decoders may register
// metrics on the reporter, so it should be called only once per
// reporter. It panics on unknown decoders.
func New(name string, r *reporter.Reporter) Decoder {
	return registeredDecoders[name](r)
}

// registeredDecoders is the list of all registered decoders.
var registeredDecoders = map[string]NewDecoderFunc{}
