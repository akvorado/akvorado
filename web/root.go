// Package web exposes a web interface.
package web

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	netHTTP "net/http"
	"net/http/httputil"
	"net/url"

	"akvorado/http"
	"akvorado/reporter"

	"github.com/rs/zerolog"
)

//go:embed data
var rootSite embed.FS

// Component represents the web component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	config Configuration
}

// Dependencies define the dependencies of the web component.
type Dependencies struct {
	HTTP *http.Component
}

// New creates a new web component.
func New(reporter *reporter.Reporter, config Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      reporter,
		d:      &dependencies,
		config: config,
	}
	data, err := fs.Sub(rootSite, "data")
	if err != nil {
		return nil, fmt.Errorf("unable to get embedded website: %w", err)
	}
	c.d.HTTP.AddHandler("/", netHTTP.FileServer(netHTTP.FS(data)))
	if c.config.GrafanaURL != "" {
		// Provide a proxy for Grafana
		url, err := url.Parse(config.GrafanaURL)
		if err != nil {
			return nil, fmt.Errorf("unable to parse Grafana URL %q: %w", config.GrafanaURL, err)
		}
		proxy := httputil.NewSingleHostReverseProxy(url)
		proxy.Transport = &netHTTP.Transport{
			Proxy: nil, // Disable proxy
		}
		proxy.ErrorLog = log.New(c.r.With().
			Str("proxy", "grafana").
			Str("level", zerolog.LevelWarnValue).
			Logger(), "", 0)
		proxyHandler := netHTTP.HandlerFunc(
			func(w netHTTP.ResponseWriter, r *netHTTP.Request) {
				proxy.ServeHTTP(w, r)
			})
		c.d.HTTP.AddHandler("/grafana/", proxyHandler)
	}
	return &c, nil
}
