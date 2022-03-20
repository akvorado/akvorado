// Package clickhouse handles housekeeping for the Clickhouse database.
package clickhouse

import (
	"akvorado/http"
	"akvorado/reporter"
)

// Component represents the Kafka exporter.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	config Configuration
}

// Dependencies define the dependencies of the Kafka exporter.
type Dependencies struct {
	HTTP *http.Component
}

// New creates a new Clickhouse component.
func New(reporter *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      reporter,
		d:      &dependencies,
		config: configuration,
	}
	if err := c.registerHTTPHandlers(); err != nil {
		return nil, err
	}
	return &c, nil
}
