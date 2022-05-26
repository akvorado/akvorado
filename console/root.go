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
	"time"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"

	"github.com/benbjohnson/clock"
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

	flowsTables     []flowsTable
	flowsTablesLock sync.RWMutex

	metrics struct {
		clickhouseQueries *reporter.CounterVec
	}
}

// Dependencies define the dependencies of the console component.
type Dependencies struct {
	Daemon       daemon.Component
	HTTP         *http.Component
	ClickHouseDB *clickhousedb.Component
	Clock        clock.Clock
}

// New creates a new console component.
func New(r *reporter.Reporter, config Configuration, dependencies Dependencies) (*Component, error) {
	if dependencies.Clock == nil {
		dependencies.Clock = clock.New()
	}
	c := Component{
		r:           r,
		d:           &dependencies,
		config:      config,
		flowsTables: []flowsTable{{"flows", 0, time.Time{}}},
	}

	c.d.Daemon.Track(&c.t, "console")

	c.metrics.clickhouseQueries = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "clickhouse_queries_total",
			Help: "Number of requests to ClickHouse.",
		}, []string{"table"},
	)
	return &c, nil
}

// Start starts the console component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting console component")

	c.d.HTTP.AddHandler("/", netHTTP.HandlerFunc(c.assetsHandlerFunc))
	c.d.HTTP.GinRouter.GET("/api/v0/console/docs/:name", c.docsHandlerFunc)
	c.d.HTTP.GinRouter.GET("/api/v0/console/widget/flow-last", c.widgetFlowLastHandlerFunc)
	c.d.HTTP.GinRouter.GET("/api/v0/console/widget/flow-rate", c.widgetFlowRateHandlerFunc)
	c.d.HTTP.GinRouter.GET("/api/v0/console/widget/exporters", c.widgetExportersHandlerFunc)
	c.d.HTTP.GinRouter.GET("/api/v0/console/widget/top/:name", c.widgetTopHandlerFunc)
	c.d.HTTP.GinRouter.GET("/api/v0/console/widget/graph", c.widgetGraphHandlerFunc)
	c.d.HTTP.GinRouter.POST("/api/v0/console/graph", c.graphHandlerFunc)
	c.d.HTTP.GinRouter.POST("/api/v0/console/sankey", c.sankeyHandlerFunc)
	c.d.HTTP.GinRouter.POST("/api/v0/console/filter/validate", c.filterValidateHandlerFunc)
	c.d.HTTP.GinRouter.POST("/api/v0/console/filter/complete", c.filterCompleteHandlerFunc)

	c.t.Go(func() error {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := c.refreshFlowsTables(); err != nil {
					c.r.Err(err).Msg("cannot refresh flows tables")
					continue
				}
				// Once successful, do that less often
				ticker.Reset(10 * time.Minute)
			case <-c.t.Dying():
				return nil
			}
		}
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
