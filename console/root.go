// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package console exposes a web interface.
package console

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"gopkg.in/tomb.v2"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/console/authentication"
	"akvorado/console/database"
	"akvorado/console/query"
)

// Component represents the console component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	flowsTables     []flowsTable
	flowsTablesLock sync.RWMutex

	metrics struct {
		clickhouseQueries *reporter.CounterVec
	}
}

// Dependencies define the dependencies of the console component.
type Dependencies struct {
	Daemon       daemon.Component
	HTTP         *httpserver.Component
	ClickHouseDB *clickhousedb.Component
	Clock        clock.Clock
	Auth         *authentication.Component
	Database     *database.Component
	Schema       *schema.Component
}

// New creates a new console component.
func New(r *reporter.Reporter, config Configuration, dependencies Dependencies) (*Component, error) {
	if dependencies.Clock == nil {
		dependencies.Clock = clock.New()
	}
	if err := query.Columns(config.DefaultVisualizeOptions.Dimensions).Validate(dependencies.Schema); err != nil {
		return nil, err
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

	// Static assets
	c.d.HTTP.AddHandler("/", http.HandlerFunc(c.defaultHandlerFunc))
	c.d.HTTP.AddHandler("/assets/", http.StripPrefix("/assets/", http.HandlerFunc(c.staticAssetsHandlerFunc)))
	c.d.HTTP.AddHandler("/assets/docs/", http.StripPrefix("/assets/docs/", http.HandlerFunc(c.docAssetsHandlerFunc)))
	// Dynamic assets
	endpoint := c.d.HTTP.GinRouter.Group("/api/v0/console", c.d.Auth.UserAuthentication())
	endpoint.GET("/configuration", c.configHandlerFunc)
	endpoint.GET("/docs/:name", c.docsHandlerFunc)
	endpoint.GET("/widget/flow-last", c.d.HTTP.CacheByRequestPath(5*time.Second), c.widgetFlowLastHandlerFunc)
	endpoint.GET("/widget/flow-rate", c.d.HTTP.CacheByRequestPath(5*time.Second), c.widgetFlowRateHandlerFunc)
	endpoint.GET("/widget/exporters", c.d.HTTP.CacheByRequestPath(30*time.Second), c.widgetExportersHandlerFunc)
	endpoint.GET("/widget/top/:name", c.d.HTTP.CacheByRequestPath(30*time.Second), c.widgetTopHandlerFunc)
	endpoint.GET("/widget/graph", c.d.HTTP.CacheByRequestPath(5*time.Minute), c.widgetGraphHandlerFunc)
	endpoint.POST("/graph/line", c.d.HTTP.CacheByRequestBody(c.config.CacheTTL), c.graphLineHandlerFunc)
	endpoint.POST("/graph/sankey", c.d.HTTP.CacheByRequestBody(c.config.CacheTTL), c.graphSankeyHandlerFunc)
	endpoint.POST("/graph/table-interval", c.getTableAndIntervalHandlerFunc)
	endpoint.POST("/filter/validate", c.filterValidateHandlerFunc)
	endpoint.POST("/filter/complete", c.d.HTTP.CacheByRequestBody(time.Minute), c.filterCompleteHandlerFunc)
	endpoint.GET("/filter/saved", c.filterSavedListHandlerFunc)
	endpoint.DELETE("/filter/saved/:id", c.filterSavedDeleteHandlerFunc)
	endpoint.POST("/filter/saved", c.filterSavedAddHandlerFunc)
	endpoint.GET("/user/info", c.d.Auth.UserInfoHandlerFunc)
	endpoint.GET("/user/avatar", c.d.Auth.UserAvatarHandlerFunc)

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
