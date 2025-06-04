// Package ddos implements a simple DDoS detection module.
package ddos

import (
	"encoding/json"
	"net/netip"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// Component performs basic DDoS detection on incoming flows.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	config Configuration

	stats   map[netip.Addr]*entry
	metrics metrics
}

// Dependencies are the dependencies of the DDoS component.
type Dependencies struct {
	Daemon daemon.Component
}

type entry struct {
	start   time.Time
	count   uint64
	sources map[netip.Addr]struct{}
}

// New creates a new DDoS detection component.
func New(r *reporter.Reporter, cfg Configuration, deps Dependencies) (*Component, error) {
	c := &Component{
		r:      r,
		d:      &deps,
		config: cfg,
		stats:  make(map[netip.Addr]*entry),
	}
	c.initMetrics()
	if c.config.DetectionWindow == 0 {
		c.config.DetectionWindow = 10 * time.Second
	}
	if c.config.MinFlows == 0 {
		c.config.MinFlows = 1000
	}
	return c, nil
}

// Start starts the component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting ddos component")
	return nil
}

// Stop stops the component.
func (c *Component) Stop() error {
	c.r.Info().Msg("stopping ddos component")
	return nil
}

// Process inspects a flow for potential DDoS activity.
func (c *Component) Process(flow *schema.FlowMessage) {
	if !c.config.Enabled {
		return
	}
	now := time.Now()
	dst := flow.DstAddr
	s := c.stats[dst]
	if s == nil {
		s = &entry{start: now, sources: make(map[netip.Addr]struct{})}
		c.stats[dst] = s
	}
	if now.Sub(s.start) > c.config.DetectionWindow {
		s.start = now
		s.count = 0
		s.sources = make(map[netip.Addr]struct{})
	}
	s.count++
	s.sources[flow.SrcAddr] = struct{}{}
	if s.count >= c.config.MinFlows {
		c.emitEvent(dst, s)
		s.start = now
		s.count = 0
		s.sources = make(map[netip.Addr]struct{})
	}
}

func (c *Component) emitEvent(dst netip.Addr, e *entry) {
	srcs := make([]string, 0, len(e.sources))
	for ip := range e.sources {
		srcs = append(srcs, ip.String())
	}
	event := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"type":      "ddos",
		"subtype":   "flow_spike",
		"dst_ip":    dst.String(),
		"flows":     e.count,
		"src_ips":   srcs,
		"flowspec": map[string]interface{}{
			"match": map[string]interface{}{
				"dst_prefix": dst.String() + "/32",
			},
			"actions": map[string]interface{}{
				"rate_limit": 1000,
			},
		},
	}
	b, _ := json.Marshal(event)
	c.metrics.detections.Inc()
	c.r.Info().Bytes("event", b).Msg("ddos detected")
}
