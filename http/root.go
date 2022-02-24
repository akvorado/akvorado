// Package http handles the internal web server for flowexporter.
package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"gopkg.in/tomb.v2"

	"flowexporter/daemon"
	"flowexporter/reporter"
)

// Component represents the HTTP compomenent.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	mux *http.ServeMux

	// Local address used by the HTTP server. Only valid after Start().
	Address net.Addr
}

// Dependencies define the dependencies of the HTTP component.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new HTTP component.
func New(reporter *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      reporter,
		d:      &dependencies,
		config: configuration,

		mux: http.NewServeMux(),
	}
	c.d.Daemon.Track(&c.t, "http")
	return &c, nil
}

// AddHandler registers a new handler for the web server
func (c *Component) AddHandler(location string, handler http.Handler) {
	c.mux.Handle(location, handler)
}

// Start starts the HTTP component.
func (c *Component) Start() error {
	if c.config.Listen == "" {
		return nil
	}
	server := &http.Server{
		Addr:    c.config.Listen,
		Handler: c.mux,
	}

	// Most of the time, if we have an error, it's here!
	c.r.Info().Str("listen", c.config.Listen).Msg("starting HTTP server")
	listener, err := net.Listen("tcp", c.config.Listen)
	if err != nil {
		return fmt.Errorf("unable to listen to %v: %w", c.config.Listen, err)
	}
	c.Address = listener.Addr()

	// Start serving requests
	c.t.Go(func() error {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			c.r.Err(err).Str("listen", c.config.Listen).Msg("unable to start HTTP server")
			return fmt.Errorf("unable to start HTTP server: %w", err)
		}
		return nil
	})

	// Gracefully stop when asked to
	c.t.Go(func() error {
		select {
		case <-c.t.Dying():
			c.r.Info().Msg("shutting down HTTP server")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				c.r.Err(err).Msg("unable to shutdown HTTP server")
				return fmt.Errorf("unable to shutdown HTTP server: %w", err)
			}
			return nil
		}
	})
	return nil
}

// Stop stops the HTTP component
func (c *Component) Stop() error {
	if c.config.Listen == "" {
		return nil
	}
	c.t.Kill(nil)
	return c.t.Wait()
}
