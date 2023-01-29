// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package core plumbs all the other components together.
package core

import (
	"fmt"
	"sync/atomic"
	"time"

	"gopkg.in/tomb.v2"
	"zgo.at/zcache/v2"

	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/bmp"
	"akvorado/inlet/flow"
	"akvorado/inlet/geoip"
	"akvorado/inlet/kafka"
	"akvorado/inlet/snmp"
)

// Component represents the HTTP compomenent.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	metrics metrics

	healthy            chan reporter.ChannelHealthcheckFunc
	httpFlowClients    uint32 // for dumping flows
	httpFlowChannel    chan *schema.FlowMessage
	httpFlowFlushDelay time.Duration

	classifierExporterCache  *zcache.Cache[exporterInfo, exporterClassification]
	classifierInterfaceCache *zcache.Cache[exporterAndInterfaceInfo, interfaceClassification]
	classifierErrLogger      reporter.Logger
}

// Dependencies define the dependencies of the HTTP component.
type Dependencies struct {
	Daemon daemon.Component
	Flow   *flow.Component
	SNMP   *snmp.Component
	BMP    *bmp.Component
	GeoIP  *geoip.Component
	Kafka  *kafka.Component
	HTTP   *http.Component
	Schema *schema.Component
}

// New creates a new core component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      r,
		d:      &dependencies,
		config: configuration,

		healthy:            make(chan reporter.ChannelHealthcheckFunc),
		httpFlowClients:    0,
		httpFlowChannel:    make(chan *schema.FlowMessage, 10),
		httpFlowFlushDelay: time.Second,

		classifierExporterCache:  zcache.New[exporterInfo, exporterClassification](configuration.ClassifierCacheDuration, 2*configuration.ClassifierCacheDuration),
		classifierInterfaceCache: zcache.New[exporterAndInterfaceInfo, interfaceClassification](configuration.ClassifierCacheDuration, 2*configuration.ClassifierCacheDuration),
		classifierErrLogger:      r.Sample(reporter.BurstSampler(10*time.Second, 3)),
	}
	c.d.Daemon.Track(&c.t, "inlet/core")
	c.initMetrics()
	return &c, nil
}

// Start starts the core component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting core component")
	for i := 0; i < c.config.Workers; i++ {
		workerID := i
		c.t.Go(func() error {
			return c.runWorker(workerID)
		})
	}

	c.r.RegisterHealthcheck("core", c.channelHealthcheck())
	c.d.HTTP.GinRouter.GET("/api/v0/inlet/flows", c.FlowsHTTPHandler)
	return nil
}

// runWorker starts a worker.
func (c *Component) runWorker(workerID int) error {
	c.r.Debug().Int("worker", workerID).Msg("starting core worker")

	for {
		select {
		case <-c.t.Dying():
			c.r.Debug().Int("worker", workerID).Msg("stopping core worker")
			return nil
		case cb, ok := <-c.healthy:
			if ok {
				cb(reporter.HealthcheckOK, fmt.Sprintf("worker %d ok", workerID))
			}
		case flow := <-c.d.Flow.Flows():
			if flow == nil {
				c.r.Info().Int("worker", workerID).Msg("no more flow available, stopping")
				return nil
			}

			exporter := flow.ExporterAddress.Unmap().String()
			c.metrics.flowsReceived.WithLabelValues(exporter).Inc()

			// Enrichment
			ip := flow.ExporterAddress
			if skip := c.enrichFlow(ip, exporter, flow); skip {
				continue
			}

			// Serialize flow to Protobuf
			buf := c.d.Schema.ProtobufMarshal(flow)

			// Forward to Kafka. This could block and buf is now owned by the
			// Kafka subsystem!
			c.metrics.flowsForwarded.WithLabelValues(exporter).Inc()
			c.d.Kafka.Send(exporter, buf)

			// If we have HTTP clients, send to them too
			if atomic.LoadUint32(&c.httpFlowClients) > 0 {
				select {
				case c.httpFlowChannel <- flow: // OK
				default: // Overflow, best effort and ignore
				}
			}

		}
	}
}

// Stop stops the core component.
func (c *Component) Stop() error {
	defer func() {
		close(c.httpFlowChannel)
		close(c.healthy)
		c.r.Info().Msg("core component stopped")
	}()
	c.r.Info().Msg("stopping core component")
	c.t.Kill(nil)
	return c.t.Wait()
}

func (c *Component) channelHealthcheck() reporter.HealthcheckFunc {
	return reporter.ChannelHealthcheck(c.t.Context(nil), c.healthy)
}
