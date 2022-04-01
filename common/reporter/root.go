// Package reporter is a façade for reporting duties in akvorado.
//
// Such a façade currently includes logging and metrics.
package reporter

import (
	"sync"

	"akvorado/common/reporter/logger"
	"akvorado/common/reporter/metrics"
)

// Reporter contains the state for a reporter. It also supports the
// same interface as a logger.
type Reporter struct {
	logger.Logger
	metrics *metrics.Metrics

	healthchecks     map[string]HealthcheckFunc
	healthchecksLock sync.Mutex
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

	return &Reporter{
		Logger:       l,
		metrics:      m,
		healthchecks: make(map[string]HealthcheckFunc),
	}, nil
}
