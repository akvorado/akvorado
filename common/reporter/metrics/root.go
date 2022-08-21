// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package metrics handles metrics for akvorado
//
// This is a wrapper around Prometheus Go client.
package metrics

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"akvorado/common/reporter/logger"
	"akvorado/common/reporter/stack"
)

// Metrics represents the internal state of the metric subsystem.
type Metrics struct {
	logger           logger.Logger
	config           Configuration
	registry         *prometheus.Registry
	factoryCache     map[string]*Factory
	factoryCacheLock sync.RWMutex
}

// New creates a new metric registry and setup the appropriate
// exporters. The provided prefix is used for system-wide metrics.
func New(logger logger.Logger, configuration Configuration) (*Metrics, error) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(collectors.NewGoCollector(collectors.WithGoCollections(
		collectors.GoRuntimeMemStatsCollection | collectors.GoRuntimeMetricsCollection)))
	m := Metrics{
		logger:       logger,
		config:       configuration,
		registry:     reg,
		factoryCache: make(map[string]*Factory, 0),
	}

	return &m, nil
}

// HTTPHandler returns an handler to server Prometheus metrics.
func (m *Metrics) HTTPHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		ErrorLog: promHTTPLogger{m.logger},
	})
}

func getPrefix(module string) (moduleName string) {
	if !strings.HasPrefix(module, stack.ModuleName) {
		moduleName = stack.ModuleName
	} else {
		moduleName = strings.SplitN(module, ".", 2)[0]
	}
	moduleName = strings.ReplaceAll(moduleName, "/", "_")
	moduleName = strings.ReplaceAll(moduleName, ".", "_")
	moduleName = fmt.Sprintf("%s_", moduleName)
	return
}

// Factory returns a factory to register new metrics with promauto. It
// includes the module as an automatic prefix. This method is expected
// to be called only from our own module to avoid walking the stack
// too often. It uses a cache to speedup things a little bit.
func (m *Metrics) Factory(skipCallstack int) *Factory {
	callStack := stack.Callers()
	call := callStack[1+skipCallstack] // Trial and error, there is a test to check it works
	module := call.FunctionName()

	// Hotpath
	if factory := func() *Factory {
		m.factoryCacheLock.RLock()
		defer m.factoryCacheLock.RUnlock()
		if factory, ok := m.factoryCache[module]; ok {
			return factory
		}
		return nil
	}(); factory != nil {
		return factory
	}

	// Slow path
	m.factoryCacheLock.Lock()
	defer m.factoryCacheLock.Unlock()
	moduleName := getPrefix(module)
	factory := Factory{
		prefix:   moduleName,
		registry: m.registry,
	}
	m.factoryCache[module] = &factory
	return &factory
}

// Desc allocates and initializes and new metric description. Like for
// factory, names are prefixed with the module name. Unlike factory,
// there is no cache.
func (m *Metrics) Desc(skipCallstack int, name, help string, variableLabels []string) *prometheus.Desc {
	callStack := stack.Callers()
	call := callStack[1+skipCallstack] // Trial and error, there is a test to check it works
	prefix := getPrefix(call.FunctionName())
	name = fmt.Sprintf("%s%s", prefix, name)
	return prometheus.NewDesc(name, help, variableLabels, nil)
}

// Collector register a custom collector.
func (m *Metrics) Collector(c prometheus.Collector) {
	m.registry.MustRegister(c)
}

// CollectorForCurrentModule register a custom collector and prefix
// everything with the module name.
func (m *Metrics) CollectorForCurrentModule(skipCallStack int, c prometheus.Collector) {
	callStack := stack.Callers()
	call := callStack[1+skipCallStack] // Should be the same as above !
	prefix := getPrefix(call.FunctionName())
	prometheus.WrapRegistererWithPrefix(prefix, m.registry).MustRegister(c)
}
