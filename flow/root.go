// Package flow handle incoming flows (currently Netflow v9).
package flow

import (
	_ "embed" // for flow.proto
	"errors"
	"fmt"

	"gopkg.in/tomb.v2"

	"akvorado/daemon"
	"akvorado/flow/decoder"
	"akvorado/flow/input"
	"akvorado/http"
	"akvorado/reporter"
)

// Component represents the flow component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	metrics struct {
		decoderStats  *reporter.CounterVec
		decoderErrors *reporter.CounterVec
		decoderTime   *reporter.SummaryVec
	}

	// Channel for sending flows out of the package.
	outgoingFlows chan *Message

	// Inputs and decoders
	inputs   []input.Input
	decoders []decoder.Decoder
}

// Dependencies are the dependencies of the flow component.
type Dependencies struct {
	Daemon daemon.Component
	HTTP   *http.Component
}

// New creates a new flow component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	if len(configuration.Inputs) == 0 {
		return nil, errors.New("no input configured")
	}

	c := Component{
		r:             r,
		d:             &dependencies,
		config:        configuration,
		outgoingFlows: make(chan *Message),
		inputs:        make([]input.Input, len(configuration.Inputs)),
		decoders:      make([]decoder.Decoder, len(configuration.Inputs)),
	}

	// Initialize inputs
	for idx, input := range c.config.Inputs {
		var err error
		c.inputs[idx], err = input.Config.New(r, c.d.Daemon)
		if err != nil {
			return nil, err
		}
	}

	// Initialize decoders (at most once each)
	var alreadyInitialized = map[string]decoder.Decoder{}
	for idx, input := range c.config.Inputs {
		dec, ok := alreadyInitialized[input.Decoder]
		if ok {
			c.decoders[idx] = dec
			continue
		}
		decoderfunc, ok := decoders[input.Decoder]
		if !ok {
			return nil, fmt.Errorf("unknown decoder %q", input.Decoder)
		}
		dec = decoderfunc(r)
		alreadyInitialized[input.Decoder] = dec
		c.decoders[idx] = dec
	}

	// Metrics
	c.metrics.decoderStats = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "decoder_count",
			Help: "Decoder processed count.",
		},
		[]string{"name"},
	)
	c.metrics.decoderErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "decoder_error_count",
			Help: "Decoder processed error count.",
		},
		[]string{"name"},
	)
	c.metrics.decoderTime = c.r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "summary_decoding_time_seconds",
			Help:       "Decoding time summary.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"name"},
	)

	c.d.Daemon.Track(&c.t, "flow")
	c.initHTTP()
	return &c, nil
}

// Flows returns a channel to receive flows.
func (c *Component) Flows() <-chan *Message {
	return c.outgoingFlows
}

// Start starts the flow component.
func (c *Component) Start() error {
	for idx, input := range c.inputs {
		decoder := c.decoders[idx]
		ch, err := input.Start()
		stopper := input.Stop
		if err != nil {
			return err
		}
		c.t.Go(func() error {
			defer stopper()
			for {
				select {
				case <-c.t.Dying():
					return nil
				case infl := <-ch:
					c.decodeWith(decoder, infl)
				}
			}
		})
	}
	return nil
}

// sendFlow transmits received flows to the next component
func (c *Component) sendFlow(fmsg *Message) {
	select {
	case <-c.t.Dying():
		return
	case c.outgoingFlows <- fmsg:
	}
}

// Stop stops the flow component
func (c *Component) Stop() error {
	defer func() {
		close(c.outgoingFlows)
		c.r.Info().Msg("flow component stopped")
	}()
	c.r.Info().Msg("stopping flow component")
	c.t.Kill(nil)
	return c.t.Wait()
}
