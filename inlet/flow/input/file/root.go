// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package file handles use files as data input (for testing)
package file

import (
	"errors"
	"io/ioutil"
	"net"
	"time"

	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
	"akvorado/inlet/flow/decoder"
	"akvorado/inlet/flow/input"
)

// Input represents the state of a file input.
type Input struct {
	r      *reporter.Reporter
	t      tomb.Tomb
	config *Configuration

	ch      chan []*decoder.FlowMessage // channel to send flows to
	decoder decoder.Decoder
}

// New instantiate a new UDP listener from the provided configuration.
func (configuration *Configuration) New(r *reporter.Reporter, daemon daemon.Component, dec decoder.Decoder) (input.Input, error) {
	if len(configuration.Paths) == 0 {
		return nil, errors.New("no paths provided for file input")
	}
	input := &Input{
		r:       r,
		config:  configuration,
		ch:      make(chan []*decoder.FlowMessage),
		decoder: dec,
	}
	daemon.Track(&input.t, "inlet/flow/input/file")
	return input, nil
}

// Start starts listening to the provided UDP socket and producing flows.
func (in *Input) Start() (<-chan []*decoder.FlowMessage, error) {
	in.r.Info().Msg("file input starting")
	in.t.Go(func() error {
		for idx := 0; true; idx++ {
			path := in.config.Paths[idx%len(in.config.Paths)]
			data, err := ioutil.ReadFile(path)
			if err != nil {
				in.r.Err(err).Str("path", path).Msg("unable to read path")
				return err
			}
			flows := in.decoder.Decode(decoder.RawFlow{
				TimeReceived: time.Now(),
				Payload:      data,
				Source:       net.ParseIP("127.0.0.1"),
			})
			if len(flows) == 0 {
				continue
			}
			select {
			case <-in.t.Dying():
				return nil
			case in.ch <- flows:
			}
		}
		return nil
	})
	return in.ch, nil
}

// Stop stops the UDP listeners
func (in *Input) Stop() error {
	defer func() {
		close(in.ch)
		in.r.Info().Msg("file input stopped")
	}()
	in.t.Kill(nil)
	return in.t.Wait()
}
