// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package daemon will handle daemon-related operations: readiness,
// watchdog, exit, reexec... Currently, only exit is implemented as
// other operations do not mean much when running in Docker.
package daemon

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"gopkg.in/tomb.v2"

	"akvorado/common/reporter"
)

// Component is the interface the daemon component provides.
type Component interface {
	Start() error
	Stop() error
	Reexec()
	FinishReexec()
	Track(t *tomb.Tomb, who string)

	// Lifecycle
	Terminated() <-chan struct{}
	Terminate()
}

// realComponent is a non-mock implementation of the Component
// interface.
type realComponent struct {
	r            *reporter.Reporter
	tombs        []tombWithOrigin
	shouldReexec atomic.Bool

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
	// Listen for tombs
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
	// On signal, terminate or reexec
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals,
			syscall.SIGINT,
			syscall.SIGTERM)
		select {
		case s := <-signals:
			c.r.Debug().Stringer("signal", s).Msg("signal received")
			switch s {
			case syscall.SIGINT, syscall.SIGTERM:
				c.r.Info().Msg("quitting")
				c.Terminate()
				signal.Stop(signals)
			}
		case <-c.Terminated():
			// Do nothing.
		}
	}()
	return nil
}

// Stop will stop the component.
func (c *realComponent) Stop() error {
	c.Terminate()
	return nil
}

// Reexec will reexecute the current process with the same arguments.
func (c *realComponent) Reexec() {
	c.shouldReexec.Store(true)
	c.Terminate()
}

// FinishReexec should be called just before exiting to trigger the real reexec.
func (c *realComponent) FinishReexec() {
	if c.shouldReexec.Load() {
		executable, err := os.Executable()
		if err != nil {
			c.r.Err(err).Msg("cannot get executable name")
			return
		}

		env := os.Environ()
		args := append([]string{executable}, os.Args[1:]...)
		c.r.Info().Strs("args", args).Msg("reexec in progress")
		if err := syscall.Exec(executable, args, env); err != nil {
			c.r.Err(err).Msg("cannot reexec")
		}
	}
}

// Add a new tomb to be tracked. This is only used before Start().
func (c *realComponent) Track(t *tomb.Tomb, who string) {
	c.tombs = append(c.tombs, tombWithOrigin{
		tomb:   t,
		origin: who,
	})
}
