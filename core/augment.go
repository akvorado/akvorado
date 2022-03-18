package core

import (
	"net"
	"time"

	"golang.org/x/time/rate"

	"akvorado/flow"
	"akvorado/snmp"
)

// AugmentFlow adds more data to a flow.
func (c *Component) AugmentFlow(sampler string, flow *flow.FlowMessage) (skip bool) {
	errLimiter := rate.NewLimiter(rate.Every(time.Minute), 10)
	if flow.InIf != 0 {
		samplerName, iface, err := c.d.Snmp.Lookup(sampler, uint(flow.InIf))
		if err != nil {
			if err != snmp.ErrCacheMiss && errLimiter.Allow() {
				c.r.Err(err).Str("sampler", sampler).Msg("unable to query SNMP cache")
			}
			c.metrics.flowsErrors.WithLabelValues(sampler, err.Error()).Inc()
			skip = true
		} else {
			flow.SamplerName = samplerName
			flow.InIfName = iface.Name
			flow.InIfDescription = iface.Description
			flow.InIfSpeed = uint32(iface.Speed)
		}
	}
	if flow.OutIf != 0 {
		samplerName, iface, err := c.d.Snmp.Lookup(sampler, uint(flow.OutIf))
		if err != nil {
			// Only register a cache miss if we don't have one.
			// TODO: maybe we could do one SNMP query for both interfaces.
			if !skip {
				if err != snmp.ErrCacheMiss && errLimiter.Allow() {
					c.r.Err(err).Str("sampler", sampler).Msg("unable to query SNMP cache")
				}
				c.metrics.flowsErrors.WithLabelValues(sampler, err.Error()).Inc()
				skip = true
			}
		} else {
			flow.SamplerName = samplerName
			flow.OutIfName = iface.Name
			flow.OutIfDescription = iface.Description
			flow.OutIfSpeed = uint32(iface.Speed)
		}
	}
	if flow.SamplingRate == 0 {
		c.metrics.flowsErrors.WithLabelValues(sampler, "sampling rate missing").Inc()
		skip = true
	}
	if skip {
		return
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
	return
}
