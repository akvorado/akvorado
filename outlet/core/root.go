// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package core plumbs all the other components together.
package core

import (
	"time"

	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/helpers/cache"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/clickhouse"
	"akvorado/outlet/flow"
	"akvorado/outlet/kafka"
	"akvorado/outlet/metadata"
	"akvorado/outlet/routing"
)

// Component represents the HTTP compomenent.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	metrics metrics

	httpFlowClients    uint32 // for dumping flows
	httpFlowChannel    chan []byte
	httpFlowFlushDelay time.Duration

	classifierExporterCache  *cache.Cache[exporterInfo, exporterClassification]
	classifierInterfaceCache *cache.Cache[exporterAndInterfaceInfo, interfaceClassification]
	classifierErrLogger      reporter.Logger

	rateLimiter rateLimiter
}

// Dependencies define the dependencies of the HTTP component.
type Dependencies struct {
	Daemon     daemon.Component
	Flow       *flow.Component
	Metadata   *metadata.Component
	Routing    *routing.Component
	Kafka      kafka.Component
	ClickHouse clickhouse.Component
	HTTP       *httpserver.Component
	Schema     *schema.Component
}

// New creates a new core component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      r,
		d:      &dependencies,
		config: configuration,

		httpFlowClients:    0,
		httpFlowChannel:    make(chan []byte, 10),
		httpFlowFlushDelay: time.Second,

		classifierExporterCache:  cache.New[exporterInfo, exporterClassification](),
		classifierInterfaceCache: cache.New[exporterAndInterfaceInfo, interfaceClassification](),
		classifierErrLogger:      r.Sample(reporter.BurstSampler(10*time.Second, 3)),

		rateLimiter: newRateLimiter(),
	}
	c.d.Daemon.Track(&c.t, "outlet/core")
	c.initMetrics()
	return &c, nil
}

// Start starts the core component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting core component")
	c.d.Kafka.StartWorkers(c.newWorker)

	// Classifier cache expiration
	c.t.Go(func() error {
		for {
			select {
			case <-c.t.Dying():
				return nil
			case <-time.After(c.config.ClassifierCacheDuration):
				before := time.Now().Add(-c.config.ClassifierCacheDuration)
				c.classifierExporterCache.DeleteLastAccessedBefore(before)
				c.classifierInterfaceCache.DeleteLastAccessedBefore(before)
			}
		}
	})

	c.d.HTTP.GinRouter.GET("/api/v0/outlet/flows", c.FlowsHTTPHandler)
	return nil
}

// Stop stops the core component.
func (c *Component) Stop() error {
	defer func() {
		close(c.httpFlowChannel)
		c.r.Info().Msg("core component stopped")
	}()
	c.r.Info().Msg("stopping core component")
	c.d.Kafka.StopWorkers()
	c.t.Kill(nil)
	return c.t.Wait()
}
