// Package reporter is a façade for reporting duties in akvorado.
//
// Such a façade currently includes logging and metrics.
package reporter

import (
	"akvorado/reporter/logger"
	"akvorado/reporter/metrics"
)

// Reporter contains the state for a reporter. It also supports the
// same interface as a logger.
type Reporter struct {
	logger.Logger
	metrics *metrics.Metrics
}

// New creates a new reporter from a configuration.
func New(config Configuration) (*Reporter, error) {
	// Initialize logger
	l, err := logger.New(config.Logging)
	if err != nil {
		return nil, err
	}

	m, err := metrics.New(l, config.Metrics)
	if err != nil {
		return nil, err
	}

	return &Reporter{l, m}, nil
}

// Start starts the reporter component.
func (r *Reporter) Start() error {
	return nil
}

// Stop stops reporting and clean the associated resources.
func (r *Reporter) Stop() error {
	r.Info().Msg("stop reporting")
	return nil
}
