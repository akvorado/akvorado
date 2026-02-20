// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package flow handle incoming NetFlow/IPFIX/sflow flows.
package flow

import (
	"errors"
	"sync"

	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/inlet/flow/input"
	"akvorado/inlet/kafka"
)

// Component represents the flow component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	inputs      []input.Input
	payloadPool sync.Pool
}

// Dependencies are the dependencies of the flow component.
type Dependencies struct {
	Daemon daemon.Component
	HTTP   *httpserver.Component
	Kafka  *kafka.Component
}

// New creates a new flow component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	if len(configuration.Inputs) == 0 {
		return nil, errors.New("no input configured")
	}

	c := Component{
		r:      r,
		d:      &dependencies,
		config: configuration,
		inputs: make([]input.Input, len(configuration.Inputs)),
		payloadPool: sync.Pool{
			New: func() any {
				minPayload := 2000
				if helpers.Testing() {
					// Ensure we test the extension case.
					minPayload = 5
				}
				s := make([]byte, minPayload)
				return &s
			},
		},
	}

	// Initialize inputs
	for idx, input := range c.config.Inputs {
		var err error
		c.inputs[idx], err = input.Config.New(r, c.d.Daemon, c.Send(input))
		if err != nil {
			return nil, err
		}
	}

	c.d.Daemon.Track(&c.t, "inlet/flow")

	return &c, nil
}

// Send sends a raw flow to Kafka.
func (c *Component) Send(config InputConfiguration) input.SendFunc {
	return func(exporter string, flow *pb.RawFlow) {
		flow.TimestampSource = config.TimestampSource
		flow.Decoder = config.Decoder
		flow.UseSourceAddress = config.UseSrcAddrForExporterAddr
		flow.DecapsulationProtocol = config.DecapsulationProtocol
		flow.RateLimit = config.RateLimit

		// Get a payload from the pool and extend it if needed. We use a pool of
		// pointers to slice as we may have to extend the capacity of the slice.
		// We keep the original pointer to avoid an extra allocation on the heap
		// when returning the slice to the pool.
		ptr := c.payloadPool.Get().(*[]byte)
		bytes := *ptr
		n := flow.SizeVT()
		if cap(bytes) < n {
			bytes = make([]byte, n+100)
			*ptr = bytes
		}

		// Marshal to it, send it to Kafka and return it when done
		if n, err := flow.MarshalToSizedBufferVT(bytes[:n]); err == nil {
			c.d.Kafka.Send(exporter, bytes[:n], func() {
				c.payloadPool.Put(ptr)
			})
		} else {
			c.payloadPool.Put(ptr)
		}
	}
}

// Start starts the flow component.
func (c *Component) Start() error {
	for _, input := range c.inputs {
		err := input.Start()
		stopper := input.Stop
		if err != nil {
			return err
		}
		c.t.Go(func() error {
			<-c.t.Dying()
			stopper()
			return nil
		})
	}
	return nil
}

// Stop stops the flow component
func (c *Component) Stop() error {
	defer c.r.Info().Msg("flow component stopped")
	c.r.Info().Msg("stopping flow component")
	c.t.Kill(nil)
	return c.t.Wait()
}
