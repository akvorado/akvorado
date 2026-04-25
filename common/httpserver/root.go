// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package httpserver handles the internal web server for akvorado.
package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/httpserver/cachestore"
	"akvorado/common/reporter"
)

// Component represents the HTTP compomenent.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	mux         *http.ServeMux
	metrics     metrics
	address     net.Addr
	serviceName string

	// APIRouter is the router exposed for /api
	APIRouter  *Router
	cacheStore cachestore.Store
}

// Dependencies define the dependencies of the HTTP component.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new HTTP component.
func New(r *reporter.Reporter, serviceName string, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      r,
		d:      &dependencies,
		config: configuration,

		mux:         http.NewServeMux(),
		serviceName: serviceName,
		APIRouter:   NewRouter(),
	}
	c.initMetrics()
	c.d.Daemon.Track(&c.t, "common/http")
	c.APIRouter.Use(recoveryMiddleware(c.r))
	c.AddHandler("/api/", c.APIRouter)
	if configuration.Profiler {
		c.mux.HandleFunc("/debug/pprof/", pprof.Index)
		c.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		c.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		c.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		c.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		runtime.SetBlockProfileRate(int(10 * time.Millisecond.Nanoseconds())) // 1/10ms
		runtime.SetMutexProfileFraction(1000)                                 // 0.1%
	}
	return &c, nil
}

// AddHandler registers a new handler for the web server
func (c *Component) AddHandler(location string, handler http.Handler) {
	l := c.r.With().Str("handler", location).Logger()
	handler = hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
		level := zerolog.InfoLevel
		if r.URL.Path == "/api/v0/metrics" || r.URL.Path == "/api/v0/healthcheck" {
			level = zerolog.DebugLevel
		}
		hlog.FromRequest(r).WithLevel(level).
			Str("method", r.Method).
			Stringer("url", r.URL).
			Str("ip", r.RemoteAddr).
			Str("user-agent", r.Header.Get("User-Agent")).
			Int("status", status).
			Int("size", size).
			Dur("duration", duration).
			Msg("HTTP request")
	})(handler)
	handler = hlog.NewHandler(l)(handler)
	handler = promhttp.InstrumentHandlerResponseSize(
		c.metrics.sizes.MustCurryWith(prometheus.Labels{"handler": location}), handler)
	handler = promhttp.InstrumentHandlerCounter(
		c.metrics.requests.MustCurryWith(prometheus.Labels{"handler": location}), handler)
	handler = promhttp.InstrumentHandlerDuration(
		c.metrics.durations.MustCurryWith(prometheus.Labels{"handler": location}), handler)
	handler = promhttp.InstrumentHandlerInFlight(c.metrics.inflights, handler)

	c.mux.Handle(location, handler)
}

// recoveryMiddleware returns a middleware that recovers from panics raised
// inside handlers, logs them and returns a 500.
func recoveryMiddleware(r *reporter.Reporter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if r != nil {
						r.Error().Interface("panic", rec).Msg("HTTP handler panic")
					}
					w.WriteHeader(http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, req)
		})
	}
}

// Start starts the HTTP component.
func (c *Component) Start() error {
	var err error
	c.cacheStore, err = c.config.Cache.Config.New()
	if err != nil {
		return err
	}
	server := &http.Server{Handler: c.mux}

	for _, lc := range []struct {
		network string
		address string
		fatal   bool
	}{
		// The regular one.
		{
			network: "tcp",
			address: c.config.Listen,
			fatal:   true,
		},
		// Using abstract unix sockets for healthchecking
		{
			network: "unix",
			address: "@akvorado",
			fatal:   false,
		},
		{
			network: "unix",
			address: fmt.Sprintf("@akvorado/%s", c.serviceName),
			fatal:   false,
		},
	} {
		if lc.address == "" {
			continue
		}
		if lc.network == "unix" && runtime.GOOS != "linux" {
			continue
		}

		// Most of the time, if we have an error, it's here!
		c.r.Info().Str("listen", lc.address).Msg("starting HTTP server")
		listener, err := net.Listen(lc.network, lc.address)
		if err != nil {
			if lc.fatal {
				return fmt.Errorf("unable to listen to %v: %w", c.config.Listen, err)
			}
			c.r.Info().Err(err).Msg("cannot start HTTP server")
			continue
		}
		if lc.network == "tcp" {
			c.address = listener.Addr()
		}
		server.Addr = listener.Addr().String()

		// Serve requests
		c.t.Go(func() error {
			if err := server.Serve(listener); err != http.ErrServerClosed {
				c.r.Err(err).Str("listen", lc.address).Msg("unable to start HTTP server")
				return fmt.Errorf("unable to start HTTP server: %w", err)
			}
			return nil
		})
		// Gracefully stop when asked to
		c.t.Go(func() error {
			<-c.t.Dying()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				c.r.Err(err).Msg("unable to shutdown HTTP server")
				return fmt.Errorf("unable to shutdown HTTP server: %w", err)
			}
			return nil
		})
	}

	// In case we have no server
	c.t.Go(func() error {
		<-c.t.Dying()
		return nil
	})

	return nil
}

// Stop stops the HTTP component
func (c *Component) Stop() error {
	c.r.Info().Msg("stopping HTTP component")
	defer c.r.Info().Msg("HTTP component stopped")
	c.t.Kill(nil)
	err := c.t.Wait()
	if c.cacheStore != nil {
		if cerr := c.cacheStore.Close(); cerr != nil {
			c.r.Err(cerr).Msg("cannot close cache store")
		}
	}
	return err
}

// LocalAddr returns the address the HTTP server is listening to.
func (c *Component) LocalAddr() net.Addr {
	return c.address
}

func init() {
	// Disable proxy for client
	http.DefaultTransport.(*http.Transport).Proxy = nil
	http.DefaultClient.Timeout = 30 * time.Second
}
