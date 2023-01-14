// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"net/http"
	"time"

	"akvorado/common/schema"

	"github.com/gin-gonic/gin"
)

// Configuration describes the configuration for the console component.
type Configuration struct {
	// ServeLiveFS serve files from the filesystem instead of the embedded versions.
	ServeLiveFS bool `yaml:"-"`
	// Version is the version to display to the user.
	Version string `yaml:"-"`
	// DefaultVisualizeOptions define some defaults for the "visualize" tab.
	DefaultVisualizeOptions VisualizeOptionsConfiguration `validate:"dive"`
	// HomepageTopWidgets defines the list of widgets to display on the home page.
	HomepageTopWidgets []string `validate:"dive,oneof=src-as dst-as src-country dst-country exporter protocol etype src-port dst-port"`
	// DimensionsLimit put an upper limit to the number of dimensions to return.
	DimensionsLimit int `validate:"min=10"`
	// CacheTTL tells how long to keep the most costly requests in cache.
	CacheTTL time.Duration `validate:"min=5s"`
}

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
	Dimensions []queryColumn `json:"dimensions"`
	// Limit is the default limit to use
	Limit int `json:"limit" validate:"min=5"`
}

// DefaultConfiguration represents the default configuration for the console component.
func DefaultConfiguration() Configuration {
	return Configuration{
		DefaultVisualizeOptions: VisualizeOptionsConfiguration{
			GraphType:  "stacked",
			Start:      "6 hours ago",
			End:        "now",
			Filter:     "InIfBoundary = external",
			Dimensions: []queryColumn{queryColumn(schema.ColumnSrcAS)},
			Limit:      10,
		},
		HomepageTopWidgets: []string{"src-as", "src-port", "protocol", "src-country", "etype"},
		DimensionsLimit:    50,
		CacheTTL:           30 * time.Minute,
	}
}

func (c *Component) configHandlerFunc(gc *gin.Context) {
	dimensions := []string{}
	for _, column := range schema.Flows.Columns() {
		if column.ConsoleNotDimension {
			continue
		}
		dimensions = append(dimensions, column.Name)
	}
	gc.JSON(http.StatusOK, gin.H{
		"version":                 c.config.Version,
		"defaultVisualizeOptions": c.config.DefaultVisualizeOptions,
		"dimensionsLimit":         c.config.DimensionsLimit,
		"homepageTopWidgets":      c.config.HomepageTopWidgets,
		"dimensions":              dimensions,
	})
}
