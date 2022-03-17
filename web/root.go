// Package web exposes a web interface.
package web

import (
	"embed"
	"fmt"
	"io/fs"
	netHTTP "net/http"

	"akvorado/http"
	"akvorado/reporter"
)

//go:embed data
var rootSite embed.FS

// Component represents the web component.
type Component struct {
	r *reporter.Reporter
	d *Dependencies
}

// Dependencies define the dependencies of the web component.
type Dependencies struct {
	HTTP *http.Component
}

// New creates a new web component.
func New(reporter *reporter.Reporter, dependencies Dependencies) (*Component, error) {
	c := Component{
		r: reporter,
		d: &dependencies,
	}
	data, err := fs.Sub(rootSite, "data")
	if err != nil {
		return nil, fmt.Errorf("unable to get embedded website: %w", err)
	}
	c.d.HTTP.AddHandler("/", netHTTP.FileServer(netHTTP.FS(data)))
	return &c, nil
}
