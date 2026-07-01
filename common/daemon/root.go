// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package daemon will handle daemon-related operations: readiness,
// watchdog, exit, reexec... Currently, only exit is implemented as
// other operations do not mean much when running in Docker.
package daemon

import (
	"os"
	"os/signal"
	"syscall"

	"gopkg.in/tomb.v2"

	"akvorado/common/reporter"
)

// Component is the interface the daemon component provides.
type Component interface {
	Start() error
	Stop() error
	Track(t *tomb.Tomb, who string)

	// Lifecycle
	Terminated() <-chan struct{}
	Terminate()
}

// realComponent is a non-mock implementation of the Component
// interface.
type realComponent struct {
	r     *reporter.Reporter
	tombs []tombWithOrigin

	lifecycleComponent
}

// tombWithOrigin stores a reference to a tomb and its origin
type tombWithOrigin struct {
	tomb   *tomb.Tomb
	origin string
}

// New will create a new daemon component.
func New(r *reporter.Reporter) (Component, error) {
	return &realComponent{
		r: r,
		lifecycleComponent: lifecycleComponent{
			terminateChannel: make(chan struct{}),
		},
	}, nil
}

// Start will make the daemon component active.
func (c *realComponent) Start() error {
	c.watchTombs()
	c.watchSignals()
	return nil
}

// watchTombs starts one goroutine per tracked tomb to terminate the
// daemon when the tomb dies.
func (c *realComponent) watchTombs() {
	for _, t := range c.tombs {
		go func(t tombWithOrigin) {
			<-t.tomb.Dying()
			if t.tomb.Err() == nil {
				c.r.Debug().
					Str("component", t.origin).
					Msg("component shutting down, quitting")
			} else {
				c.r.Err(t.tomb.Err()).
					Str("component", t.origin).
					Msg("component error, quitting")
			}
			c.Terminate()
		}(t)
	}
}

// watchSignals terminates the daemon on SIGINT/SIGTERM.
func (c *realComponent) watchSignals() {
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals,
			syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(signals)
		select {
		case s := <-signals:
			c.r.Debug().Stringer("signal", s).Msg("signal received")
			switch s {
			case syscall.SIGINT, syscall.SIGTERM:
				c.r.Info().Msg("quitting")
				c.Terminate()
			}
		case <-c.Terminated():
			// Do nothing.
		}
	}()
}

// Stop will stop the component.
func (c *realComponent) Stop() error {
	c.Terminate()
	return nil
}

// Add a new tomb to be tracked. This is only used before Start().
func (c *realComponent) Track(t *tomb.Tomb, who string) {
	c.tombs = append(c.tombs, tombWithOrigin{
		tomb:   t,
		origin: who,
	})
}
