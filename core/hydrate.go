package core

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"golang.org/x/time/rate"

	"akvorado/flow"
	"akvorado/snmp"
)

// hydrateFlow adds more data to a flow.
func (c *Component) hydrateFlow(sampler string, flow *flow.FlowMessage) (skip bool) {
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
	} else {
		c.metrics.flowsErrors.WithLabelValues(sampler, "input interface missing").Inc()
		skip = true
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
	} else {
		c.metrics.flowsErrors.WithLabelValues(sampler, "output interface missing").Inc()
		skip = true
	}
	if flow.SamplingRate == 0 {
		c.metrics.flowsErrors.WithLabelValues(sampler, "sampling rate missing").Inc()
		skip = true
	}
	if skip {
		return
	}

	// Classification
	c.classifySampler(sampler, flow)
	c.classifyInterface(sampler, flow,
		flow.OutIfName, flow.OutIfDescription, flow.OutIfSpeed,
		&flow.OutIfConnectivity, &flow.OutIfProvider, &flow.OutIfBoundary)
	c.classifyInterface(sampler, flow,
		flow.InIfName, flow.InIfDescription, flow.InIfSpeed,
		&flow.InIfConnectivity, &flow.InIfProvider, &flow.InIfBoundary)

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

func (c *Component) classifySampler(ip string, flow *flow.FlowMessage) {
	if len(c.config.SamplerClassifiers) == 0 {
		return
	}
	name := flow.SamplerName
	key := fmt.Sprintf("S-%s-%s", ip, name)
	group, ok := c.classifierCache.Get(key)
	if ok {
		flow.SamplerGroup = group.(string)
		return
	}

	si := samplerInfo{IP: ip, Name: name}
	for idx, rule := range c.config.SamplerClassifiers {
		group, err := rule.exec(si)
		if err != nil {
			if c.classifierErrLimiter.Allow() {
				c.r.Err(err).
					Str("type", "sampler").
					Int("index", idx).
					Str("sampler", name).
					Msg("error executing classifier")
			}
			c.metrics.classifierErrors.WithLabelValues("sampler", strconv.Itoa(idx)).Inc()
			c.classifierCache.Set(key, "", 1)
			return
		}
		if group != "" {
			c.classifierCache.Set(key, group, 1)
			flow.SamplerGroup = group
			return
		}
	}
}

func (c *Component) classifyInterface(ip string, fl *flow.FlowMessage,
	ifName, ifDescription string, ifSpeed uint32,
	connectivity, provider *string, boundary *flow.FlowMessage_Boundary) {
	if len(c.config.InterfaceClassifiers) == 0 {
		return
	}
	key := fmt.Sprintf("I-%s-%s-%s-%s-%d", ip, fl.SamplerName, ifName, ifDescription, ifSpeed)
	if classification, ok := c.classifierCache.Get(key); ok {
		*connectivity = classification.(interfaceClassification).Connectivity
		*provider = classification.(interfaceClassification).Provider
		*boundary = convertBoundaryToProto(classification.(interfaceClassification).Boundary)
		return
	}

	si := samplerInfo{IP: ip, Name: fl.SamplerName}
	ii := interfaceInfo{Name: ifName, Description: ifDescription, Speed: ifSpeed}
	var classification interfaceClassification
	for idx, rule := range c.config.InterfaceClassifiers {
		err := rule.exec(si, ii, &classification)
		if err != nil {
			if c.classifierErrLimiter.Allow() {
				c.r.Err(err).
					Str("type", "interface").
					Int("index", idx).
					Str("sampler", fl.SamplerName).
					Str("interface", ifName).
					Msg("error executing classifier")
			}
			c.metrics.classifierErrors.WithLabelValues("interface", strconv.Itoa(idx)).Inc()
			c.classifierCache.Set(key, classification, 1)
			return
		}
		if classification.Connectivity == "" || classification.Provider == "" {
			continue
		}
		if classification.Boundary == undefinedBoundary {
			continue
		}
		break
	}
	c.classifierCache.Set(key, classification, 1)
	*connectivity = classification.Connectivity
	*provider = classification.Provider
	*boundary = convertBoundaryToProto(classification.Boundary)
}

func convertBoundaryToProto(from interfaceBoundary) flow.FlowMessage_Boundary {
	switch from {
	case externalBoundary:
		return flow.FlowMessage_EXTERNAL
	case internalBoundary:
		return flow.FlowMessage_INTERNAL
	}
	return flow.FlowMessage_UNDEFINED
}
