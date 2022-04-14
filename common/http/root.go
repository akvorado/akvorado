// Package http handles the internal web server for akvorado.
package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/hlog"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
)

// Component represents the HTTP compomenent.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	mux     *http.ServeMux
	metrics struct {
		inflights reporter.Gauge
		requests  *reporter.CounterVec
		durations *reporter.HistogramVec
		sizes     *reporter.HistogramVec
	}

	// Local address used by the HTTP server. Only valid after Start().
	Address net.Addr

	// GinRouter is the router exposed for /api
	GinRouter *gin.Engine
}

// Dependencies define the dependencies of the HTTP component.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new HTTP component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      r,
		d:      &dependencies,
		config: configuration,

		mux:       http.NewServeMux(),
		GinRouter: gin.New(),
	}
	c.d.Daemon.Track(&c.t, "common/http")

	c.metrics.inflights = c.r.Gauge(
		reporter.GaugeOpts{
			Name: "inflight_requests",
			Help: "Number of requests currently being served by the HTTP server.",
		},
	)
	c.metrics.requests = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "requests_total",
			Help: "Number of requests handled by an handler.",
		}, []string{"handler", "code", "method"},
	)
	c.metrics.durations = c.r.HistogramVec(
		reporter.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "Latencies for served requests.",
			Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
		}, []string{"handler", "method"},
	)
	c.metrics.sizes = c.r.HistogramVec(
		reporter.HistogramOpts{
			Name:    "response_size_bytes",
			Help:    "Response sizes for requests.",
			Buckets: []float64{200, 500, 1000, 1500, 5000},
		}, []string{"handler", "method"},
	)

	c.GinRouter.Use(gin.Recovery())
	c.AddHandler("/api/", c.GinRouter)
	if configuration.Profiler {
		c.mux.HandleFunc("/debug/pprof/", pprof.Index)
		c.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		c.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		c.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		c.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
	return &c, nil
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rw *responseWriter) Status() int {
	return rw.status
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
	rw.wroteHeader = true
	return
}

// AddHandler registers a new handler for the web server
func (c *Component) AddHandler(location string, handler http.Handler) {
	l := c.r.With().Str("handler", location).Logger()
	handler = hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
		hlog.FromRequest(r).Info().
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
	c.r.Info().Msg("stopping HTTP component")
	defer c.r.Info().Msg("HTTP component stopped")
	c.t.Kill(nil)
	return c.t.Wait()
}

func init() {
	// Disable proxy for client
	http.DefaultTransport.(*http.Transport).Proxy = nil
	http.DefaultClient.Timeout = 30 * time.Second
	gin.SetMode(gin.ReleaseMode)
}
