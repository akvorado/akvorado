// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package file handles use files as data input (for testing)
package file

import (
	"errors"
	"net"
	"os"
	"time"

	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/inlet/flow/input"
)

// Input represents the state of a file input.
type Input struct {
	r      *reporter.Reporter
	t      tomb.Tomb
	config Configuration
	send   input.SendFunc
}

var (
	_ input.Input         = &Input{}
	_ input.Configuration = Configuration{}
)

// New instantiate a new UDP listener from the provided configuration.
func (configuration Configuration) New(r *reporter.Reporter, daemon daemon.Component, send input.SendFunc) (input.Input, error) {
	if len(configuration.Paths) == 0 {
		return nil, errors.New("no paths provided for file input")
	}
	input := &Input{
		r:      r,
		config: configuration,
		send:   send,
	}
	daemon.Track(&input.t, "inlet/flow/input/file")
	return input, nil
}

// Start starts streaming files to produce flake flows in a loop.
func (in *Input) Start() error {
	in.r.Info().Msg("file input starting")
	in.t.Go(func() error {
		count := uint(0)
		payload := make([]byte, 9000)
		flow := pb.RawFlow{}
		for idx := 0; true; idx++ {
			if in.config.MaxFlows > 0 && count >= in.config.MaxFlows {
				<-in.t.Dying()
				return nil
			}

			path := in.config.Paths[idx%len(in.config.Paths)]
			data, err := os.ReadFile(path)
			if err != nil {
				in.r.Err(err).Str("path", path).Msg("unable to read path")
				return err
			}

			// Mimic the way it works with UDP
			n := copy(payload, data)
			flow.Reset()
			flow.TimeReceived = uint64(time.Now().Unix())
			flow.Payload = payload[:n]
			flow.SourceAddress = net.ParseIP("127.0.0.1").To16()

			in.send("127.0.0.1", &flow)
			count++
			select {
			case <-in.t.Dying():
				return nil
			default:
			}
		}
		return nil
	})
	return nil
}

// Stop stops the UDP listeners
func (in *Input) Stop() error {
	defer in.r.Info().Msg("file input stopped")
	in.t.Kill(nil)
	return in.t.Wait()
}
