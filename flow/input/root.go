package input

import (
	"net"
	"time"

	"akvorado/daemon"
	"akvorado/reporter"
)

// Input is the interface any input should meet
type Input interface {
	// Start instructs an input to start producing flows on the returned channel.
	Start() (<-chan Flow, error)
	// Stop instructs the input to stop producing flows.
	Stop() error
}

// Flow is an incoming flow from an input.
type Flow struct {
	TimeReceived time.Time
	Payload      []byte
	Source       net.IP
}

// Configuration the interface for the configuration for an input module.
type Configuration interface {
	// New instantiantes a new input from its configuration.
	New(r *reporter.Reporter, daemon daemon.Component) (Input, error)
}
