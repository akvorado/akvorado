// Package core plumbs all the other components together.
package core

import (
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/time/rate"
	"gopkg.in/tomb.v2"

	"akvorado/daemon"
	"akvorado/flow"
	"akvorado/geoip"
	"akvorado/http"
	"akvorado/kafka"
	"akvorado/reporter"
	"akvorado/snmp"
)

// Component represents the HTTP compomenent.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	metrics metrics

	healthy            chan chan<- bool // for healthcheck
	httpFlowClients    uint32           // for dumping flows
	httpFlowChannel    chan *flow.FlowMessage
	httpFlowFlushDelay time.Duration
}

// Dependencies define the dependencies of the HTTP component.
type Dependencies struct {
	Daemon daemon.Component
	Flow   *flow.Component
	Snmp   *snmp.Component
	GeoIP  *geoip.Component
	Kafka  *kafka.Component
	HTTP   *http.Component
}

// New creates a new core component.
func New(reporter *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      reporter,
		d:      &dependencies,
		config: configuration,

		healthy:            make(chan chan<- bool),
		httpFlowClients:    0,
		httpFlowChannel:    make(chan *flow.FlowMessage, 10),
		httpFlowFlushDelay: time.Second,
	}
	c.d.Daemon.Track(&c.t, "core")
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

	c.d.HTTP.AddHandler("/healthcheck", c.HealthcheckHTTPHandler())
	c.d.HTTP.AddHandler("/flows", c.FlowsHTTPHandler())
	return nil
}

// runWorker starts a worker.
func (c *Component) runWorker(workerID int) error {
	c.r.Debug().Int("worker", workerID).Msg("starting core worker")

	errLimiter := rate.NewLimiter(rate.Every(time.Minute), 10)
	for {
		select {
		case <-c.t.Dying():
			c.r.Debug().Int("worker", workerID).Msg("stopping core worker")
			return nil
		case answerChan := <-c.healthy:
			answerChan <- true
		case flow := <-c.d.Flow.Flows():
			if flow == nil {
				c.r.Warn().Int("worker", workerID).Msg("no more flow available, stopping")
				return errors.New("no more flow available")
			}
			host := net.IP(flow.SamplerAddress).String()
			c.metrics.flowsReceived.WithLabelValues(host).Inc()

			// Add interface names
			cacheMiss := false
			if flow.InIf != 0 {
				iface, err := c.d.Snmp.Lookup(host, uint(flow.InIf))
				if err != nil {
					if err != snmp.ErrCacheMiss && errLimiter.Allow() {
						c.r.Err(err).Str("host", host).Msg("unable to query SNMP cache")
					}
					c.metrics.flowsErrors.WithLabelValues(host, err.Error()).Inc()
					cacheMiss = true
				} else {
					flow.InIfName = iface.Name
					flow.InIfDescription = iface.Description
				}
			}
			if flow.OutIf != 0 {
				iface, err := c.d.Snmp.Lookup(host, uint(flow.OutIf))
				if err != nil {
					// Only register a cache miss if we don't have one.
					// TODO: maybe we could do one SNMP query for both interfaces.
					if !cacheMiss {
						if err != snmp.ErrCacheMiss && errLimiter.Allow() {
							c.r.Err(err).Str("host", host).Msg("unable to query SNMP cache")
						}
						c.metrics.flowsErrors.WithLabelValues(host, err.Error()).Inc()
						cacheMiss = true
					}
				} else {
					flow.OutIfName = iface.Name
					flow.OutIfDescription = iface.Description
				}
			}
			if cacheMiss {
				continue
			}

			// Add GeoIP
			if flow.SrcAS == 0 {
				flow.SrcAS = c.d.GeoIP.LookupASN(net.IP(flow.SrcAddr))
			}
			if flow.DstAS == 0 {
				flow.DstAS = c.d.GeoIP.LookupASN(net.IP(flow.DstAddr))
			}
			flow.SrcCountry = c.d.GeoIP.LookupCountry(net.IP(flow.SrcAddr))
			flow.DstCountry = c.d.GeoIP.LookupCountry(net.IP(flow.DstAddr))

			// Serialize flow (use length-prefixed protobuf)
			buf := proto.NewBuffer([]byte{})
			err := buf.EncodeMessage(flow)
			if err != nil {
				if errLimiter.Allow() {
					c.r.Err(err).Str("host", host).Msg("unable to serialize flow")
				}
				c.metrics.flowsErrors.WithLabelValues(host, err.Error()).Inc()
				continue
			}

			// Forward to Kafka
			if err := c.d.Kafka.Send(host, buf.Bytes()); err != nil {
				if errLimiter.Allow() {
					c.r.Err(err).Str("host", host).Msg("unable to send flow to Kafka")
				}
				c.metrics.flowsErrors.WithLabelValues(host, err.Error()).Inc()
				continue
			}
			c.metrics.flowsForwarded.WithLabelValues(host).Inc()

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
	defer close(c.healthy)
	defer close(c.httpFlowChannel)
	c.r.Info().Msg("stopping core component")
	defer c.r.Info().Msg("core component stopped")
	c.t.Kill(nil)
	return c.t.Wait()
}
