// Package web exposes a web interface.
package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	netHTTP "net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/rs/zerolog"
	"gopkg.in/tomb.v2"

	"akvorado/http"
	"akvorado/reporter"
)

// Component represents the web component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	templates     map[string]*template.Template
	templatesLock sync.RWMutex
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
	if err := c.loadTemplates(); err != nil {
		return nil, err
	}

	// Grafana proxy
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

	c.d.HTTP.AddHandler("/docs/", netHTTP.HandlerFunc(c.docsHandlerFunc))
	c.d.HTTP.AddHandler("/assets/", netHTTP.HandlerFunc(c.assetsHandlerFunc))

	return &c, nil
}

// Start starts the web component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting web component")
	if err := c.watchTemplates(); err != nil {
		return err
	}
	c.t.Go(func() error {
		select {
		case <-c.t.Dying():
			return nil
		}
	})
	return nil
}

// Stop stops the web component.
func (c *Component) Stop() error {
	c.r.Info().Msg("stopping web component")
	defer c.r.Info().Msg("web component stopped")
	c.t.Kill(nil)
	return c.t.Wait()
}

// embedOrLiveFS returns a subset of the provided embedded filesystem,
// except if the component is configured to use the live filesystem.
// Then, it returns the provided tree.
func (c *Component) embedOrLiveFS(embed fs.FS, p string) fs.FS {
	var fileSystem fs.FS
	if c.config.ServeLiveFS {
		_, src, _, _ := runtime.Caller(0)
		fileSystem = os.DirFS(filepath.Join(path.Dir(src), p))
	} else {
		var err error
		fileSystem, err = fs.Sub(embed, p)
		if err != nil {
			panic(err)
		}
	}
	return fileSystem
}
