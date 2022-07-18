// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"akvorado/common/reporter"
	"akvorado/inlet/flow"
	"akvorado/inlet/flow/decoder"
	"akvorado/inlet/snmp"
)

// hydrateFlow adds more data to a flow.
func (c *Component) hydrateFlow(exporter string, flow *flow.Message) (skip bool) {
	errLogger := c.r.Sample(reporter.BurstSampler(time.Minute, 10))

	if flow.InIf != 0 {
		exporterName, iface, err := c.d.Snmp.Lookup(exporter, uint(flow.InIf))
		if err != nil {
			if err != snmp.ErrCacheMiss {
				errLogger.Err(err).Str("exporter", exporter).Msg("unable to query SNMP cache")
			}
			c.metrics.flowsErrors.WithLabelValues(exporter, err.Error()).Inc()
			skip = true
		} else {
			flow.ExporterName = exporterName
			flow.InIfName = iface.Name
			flow.InIfDescription = iface.Description
			flow.InIfSpeed = uint32(iface.Speed)
		}
	}

	if flow.OutIf != 0 {
		exporterName, iface, err := c.d.Snmp.Lookup(exporter, uint(flow.OutIf))
		if err != nil {
			// Only register a cache miss if we don't have one.
			// TODO: maybe we could do one SNMP query for both interfaces.
			if !skip {
				if err != snmp.ErrCacheMiss {
					errLogger.Err(err).Str("exporter", exporter).Msg("unable to query SNMP cache")
				}
				c.metrics.flowsErrors.WithLabelValues(exporter, err.Error()).Inc()
				skip = true
			}
		} else {
			flow.ExporterName = exporterName
			flow.OutIfName = iface.Name
			flow.OutIfDescription = iface.Description
			flow.OutIfSpeed = uint32(iface.Speed)
		}
	}

	// We need at least one of them.
	if flow.OutIf == 0 && flow.InIf == 0 {
		c.metrics.flowsErrors.WithLabelValues(exporter, "input and output interfaces missing").Inc()
		skip = true
	}

	if flow.SamplingRate == 0 {
		if c.config.DefaultSamplingRate != 0 {
			flow.SamplingRate = uint64(c.config.DefaultSamplingRate)
		} else {
			c.metrics.flowsErrors.WithLabelValues(exporter, "sampling rate missing").Inc()
			skip = true
		}
	}

	if skip {
		return
	}

	// Classification
	c.classifyExporter(exporter, flow)
	c.classifyInterface(exporter, flow,
		flow.OutIfName, flow.OutIfDescription, flow.OutIfSpeed,
		&flow.OutIfConnectivity, &flow.OutIfProvider, &flow.OutIfBoundary)
	c.classifyInterface(exporter, flow,
		flow.InIfName, flow.InIfDescription, flow.InIfSpeed,
		&flow.InIfConnectivity, &flow.InIfProvider, &flow.InIfBoundary)

	// Add GeoIP
	if flow.SrcAS == 0 || c.config.IgnoreASNFromFlow {
		flow.SrcAS = c.d.GeoIP.LookupASN(net.IP(flow.SrcAddr))
	}
	if flow.DstAS == 0 || c.config.IgnoreASNFromFlow {
		flow.DstAS = c.d.GeoIP.LookupASN(net.IP(flow.DstAddr))
	}
	flow.SrcCountry = c.d.GeoIP.LookupCountry(net.IP(flow.SrcAddr))
	flow.DstCountry = c.d.GeoIP.LookupCountry(net.IP(flow.DstAddr))

	return
}

func (c *Component) classifyExporter(ip string, flow *flow.Message) {
	if len(c.config.ExporterClassifiers) == 0 {
		return
	}
	name := flow.ExporterName
	key := fmt.Sprintf("S-%s-%s", ip, name)
	if classification, ok := c.classifierCache.Get(key); ok {
		flow.ExporterGroup = classification.(exporterClassification).Group
		flow.ExporterRole = classification.(exporterClassification).Role
		flow.ExporterSite = classification.(exporterClassification).Site
		flow.ExporterRegion = classification.(exporterClassification).Region
		flow.ExporterTenant = classification.(exporterClassification).Tenant
		return
	}

	si := exporterInfo{IP: ip, Name: name}
	var classification exporterClassification
	for idx, rule := range c.config.ExporterClassifiers {
		if err := rule.exec(si, &classification); err != nil {
			c.classifierErrLogger.Err(err).
				Str("type", "exporter").
				Int("index", idx).
				Str("exporter", name).
				Msg("error executing classifier")
			c.metrics.classifierErrors.WithLabelValues("exporter", strconv.Itoa(idx)).Inc()
			c.classifierCache.Set(key, classification, 1)
			return
		}
		if classification.Group == "" || classification.Role == "" || classification.Site == "" || classification.Region == "" || classification.Tenant == "" {
			continue
		}
		break
	}
	c.classifierCache.Set(key, classification, 1)
	flow.ExporterGroup = classification.Group
	flow.ExporterRole = classification.Role
	flow.ExporterSite = classification.Site
	flow.ExporterRegion = classification.Region
	flow.ExporterTenant = classification.Tenant
}

func (c *Component) classifyInterface(ip string, fl *flow.Message,
	ifName, ifDescription string, ifSpeed uint32,
	connectivity, provider *string, boundary *decoder.FlowMessage_Boundary) {
	if len(c.config.InterfaceClassifiers) == 0 {
		return
	}
	key := fmt.Sprintf("I-%s-%s-%s-%s-%d", ip, fl.ExporterName, ifName, ifDescription, ifSpeed)
	if classification, ok := c.classifierCache.Get(key); ok {
		*connectivity = classification.(interfaceClassification).Connectivity
		*provider = classification.(interfaceClassification).Provider
		*boundary = convertBoundaryToProto(classification.(interfaceClassification).Boundary)
		return
	}

	si := exporterInfo{IP: ip, Name: fl.ExporterName}
	ii := interfaceInfo{Name: ifName, Description: ifDescription, Speed: ifSpeed}
	var classification interfaceClassification
	for idx, rule := range c.config.InterfaceClassifiers {
		err := rule.exec(si, ii, &classification)
		if err != nil {
			c.classifierErrLogger.Err(err).
				Str("type", "interface").
				Int("index", idx).
				Str("exporter", fl.ExporterName).
				Str("interface", ifName).
				Msg("error executing classifier")
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

func convertBoundaryToProto(from interfaceBoundary) decoder.FlowMessage_Boundary {
	switch from {
	case externalBoundary:
		return decoder.FlowMessage_EXTERNAL
	case internalBoundary:
		return decoder.FlowMessage_INTERNAL
	}
	return decoder.FlowMessage_UNDEFINED
}
