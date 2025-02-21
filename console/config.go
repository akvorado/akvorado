// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"net/http"
	"time"

	"akvorado/common/helpers"
	"akvorado/console/query"

	"github.com/gin-gonic/gin"
)

// Configuration describes the configuration for the console component.
type Configuration struct {
	// ServeLiveFS serve files from the filesystem instead of the embedded versions.
	ServeLiveFS bool `yaml:"-"`
	// DefaultVisualizeOptions define some defaults for the "visualize" tab.
	DefaultVisualizeOptions VisualizeOptionsConfiguration
	// HomepageTopWidgets defines the list of widgets to display on the home page.
	HomepageTopWidgets []HomepageTopWidget
	// HomepageGraphFilter defines the filtering string to use for the homepage graph
	HomepageGraphFilter string
	// HomepageGraphTimeRange defines the time range to use for the homepage graph
	HomepageGraphTimeRange time.Duration `validate:"min=1m"`
	// DimensionsLimit put an upper limit to the number of dimensions to return.
	DimensionsLimit int `validate:"min=10"`
	// CacheTTL tells how long to keep the most costly requests in cache.
	CacheTTL time.Duration `validate:"min=5s"`
}

// HomepageTopWidget represents a top widget on the homepage.
type HomepageTopWidget int

const (
	// HomepageTopWidgetSrcAS shows the top source AS
	HomepageTopWidgetSrcAS HomepageTopWidget = iota + 1
	// HomepageTopWidgetDstAS shows the top destination AS
	HomepageTopWidgetDstAS
	// HomepageTopWidgetSrcCountry shows the top source countries
	HomepageTopWidgetSrcCountry
	// HomepageTopWidgetDstCountry shows the top destination countries
	HomepageTopWidgetDstCountry
	// HomepageTopWidgetSrcPort shows the top source ports
	HomepageTopWidgetSrcPort
	// HomepageTopWidgetDstPort shows the top destination ports
	HomepageTopWidgetDstPort
	// HomepageTopWidgetExporter shows the top exporters
	HomepageTopWidgetExporter
	// HomepageTopWidgetProtocol shows the top IP protocols
	HomepageTopWidgetProtocol
	// HomepageTopWidgetEtype shows the top ethernet types
	HomepageTopWidgetEtype
)

// VisualizeOptionsConfiguration defines options for the "visualize" tab.
type VisualizeOptionsConfiguration struct {
	// GraphType tells the type of the graph we request
	GraphType string `json:"graphType" validate:"oneof=stacked stacked100 lines grid sankey"`
	// Start is the start time (as a string)
	Start string `json:"start" validate:"required"`
	// End is the end time (as string)
	End string `json:"end" validate:"required"`
	// Filter  is the the filter string
	Filter string `json:"filter"`
	// Dimensions is the array of dimensions to use
	Dimensions []query.Column `json:"dimensions"`
	// Limit is the default limit to use
	Limit int `json:"limit" validate:"min=5"`
	// LimitType is the default limitType to use
	LimitType string `json:"limitType" validate:"oneof=avg max last"`
	// Bidirectional tells if a graph should be bidirectional (all except sankey)
	Bidirectional bool `json:"bidirectional"`
	// PreviousPeriod tells if a graph should display the previous period (for stacked)
	PreviousPeriod bool `json:"previousPeriod"`
}

// DefaultConfiguration represents the default configuration for the console component.
func DefaultConfiguration() Configuration {
	return Configuration{
		DefaultVisualizeOptions: VisualizeOptionsConfiguration{
			GraphType:  "stacked",
			Start:      "6 hours ago",
			End:        "now",
			Filter:     "InIfBoundary = external",
			Dimensions: []query.Column{query.NewColumn("SrcAS")},
			Limit:      10,
			LimitType:  "avg",
		},
		HomepageTopWidgets: []HomepageTopWidget{
			HomepageTopWidgetSrcAS,
			HomepageTopWidgetSrcPort,
			HomepageTopWidgetProtocol,
			HomepageTopWidgetSrcCountry,
			HomepageTopWidgetEtype,
		},
		DimensionsLimit:        50,
		CacheTTL:               3 * time.Hour,
		HomepageGraphFilter:    "InIfBoundary = 'external'",
		HomepageGraphTimeRange: 24 * time.Hour,
	}
}

func (c *Component) configHandlerFunc(gc *gin.Context) {
	dimensions := []string{}
	truncatable := []string{}
	for _, column := range c.d.Schema.Columns() {
		if column.ConsoleNotDimension || column.Disabled {
			continue
		}
		dimensions = append(dimensions, column.Name)
		if column.ConsoleTruncateIP {
			truncatable = append(truncatable, column.Name)
		}
	}
	gc.JSON(http.StatusOK, gin.H{
		"version":                 helpers.AkvoradoVersion,
		"defaultVisualizeOptions": c.config.DefaultVisualizeOptions,
		"dimensionsLimit":         c.config.DimensionsLimit,
		"dimensions":              dimensions,
		"truncatable":             truncatable,
		"homepageTopWidgets":      c.config.HomepageTopWidgets,
	})
}
