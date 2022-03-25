// Package file handles use files as data input (for testing)
package file

import (
	"errors"
	"io/ioutil"
	"net"
	"time"

	"gopkg.in/tomb.v2"

	"akvorado/daemon"
	"akvorado/flow/input"
	"akvorado/reporter"
)

// Input represents the state of a file input.
type Input struct {
	r      *reporter.Reporter
	t      tomb.Tomb
	config *Configuration
	ch     chan input.Flow // channel to send flows to
}

// New instantiate a new UDP listener from the provided configuration.
func (configuration *Configuration) New(r *reporter.Reporter, daemon daemon.Component) (input.Input, error) {
	if configuration.Paths == nil || len(configuration.Paths) == 0 {
		return nil, errors.New("no paths provided for file input")
	}
	input := &Input{
		r:      r,
		config: configuration,
		ch:     make(chan input.Flow),
	}
	daemon.Track(&input.t, "flow/input/file")
	return input, nil
}

// Start starts listening to the provided UDP socket and producing flows.
func (in *Input) Start() (<-chan input.Flow, error) {
	in.r.Info().Msg("file input starting")
	in.t.Go(func() error {
		for idx := 0; true; idx++ {
			path := in.config.Paths[idx%len(in.config.Paths)]
			data, err := ioutil.ReadFile(path)
			if err != nil {
				in.r.Err(err).Str("path", path).Msg("unable to read path")
				return err
			}
			flow := input.Flow{
				TimeReceived: time.Now(),
				Payload:      data,
				Source:       net.ParseIP("127.0.0.1"),
			}
			select {
			case <-in.t.Dying():
				return nil
			case in.ch <- flow:
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
