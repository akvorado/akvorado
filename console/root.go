// Package console exposes a web interface.
package console

import (
	"html/template"
	"io/fs"
	netHTTP "net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"

	"gopkg.in/tomb.v2"
)

// Component represents the console component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	templates     map[string]*template.Template
	templatesLock sync.RWMutex
}

// Dependencies define the dependencies of the console component.
type Dependencies struct {
	Daemon       daemon.Component
	HTTP         *http.Component
	ClickHouseDB *clickhousedb.Component
}

// New creates a new console component.
func New(reporter *reporter.Reporter, config Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      reporter,
		d:      &dependencies,
		config: config,
	}

	c.d.Daemon.Track(&c.t, "console")
	return &c, nil
}

// Start starts the console component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting console component")

	c.d.HTTP.AddHandler("/", netHTTP.HandlerFunc(c.assetsHandlerFunc))
	c.d.HTTP.GinRouter.GET("/api/v0/console/docs/:name", c.docsHandlerFunc)
	c.d.HTTP.GinRouter.GET("/api/v0/console/last-flow", c.apiLastFlowHandlerFunc)
	c.d.HTTP.GinRouter.GET("/api/v0/console/exporters", c.apiExportersHandlerFunc)

	c.t.Go(func() error {
		select {
		case <-c.t.Dying():
		}
		return nil
	})
	return nil
}

// Stop stops the console component.
func (c *Component) Stop() error {
	defer c.r.Info().Msg("console component stopped")
	c.r.Info().Msg("stopping console component")
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
