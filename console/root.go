// Package console exposes a web interface.
package console

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

	"akvorado/common/http"
	"akvorado/common/reporter"
)

// Component represents the console component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	config Configuration

	templates     map[string]*template.Template
	templatesLock sync.RWMutex
}

// Dependencies define the dependencies of the console component.
type Dependencies struct {
	HTTP *http.Component
}

// New creates a new console component.
func New(reporter *reporter.Reporter, config Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      reporter,
		d:      &dependencies,
		config: config,
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

	c.d.HTTP.AddHandler("/", netHTTP.HandlerFunc(c.assetsHandlerFunc))
	c.d.HTTP.AddHandler("/api/v0/docs/", netHTTP.HandlerFunc(c.docsHandlerFunc))

	return &c, nil
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
