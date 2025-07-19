// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package flow handle incoming Netflow/IPFIX/sflow flows.
package flow

import (
	"errors"

	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
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

	// Inputs
	inputs []input.Input
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
		if bytes, err := flow.MarshalVT(); err == nil {
			c.d.Kafka.Send(exporter, bytes)
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
