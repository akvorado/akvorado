// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"time"

	"akvorado/common/reporter"
	"akvorado/inlet/flow"
	"akvorado/inlet/flow/decoder"
	"akvorado/inlet/snmp"
)

// exporterAndInterfaceInfo aggregates both exporter info and interface info
type exporterAndInterfaceInfo struct {
	Exporter  exporterInfo
	Interface interfaceInfo
}

// enrichFlow adds more data to a flow.
func (c *Component) enrichFlow(exporterIP netip.Addr, exporterStr string, flow *flow.Message) (skip bool) {
	errLogger := c.r.Sample(reporter.BurstSampler(time.Minute, 10))

	if flow.InIf != 0 {
		exporterName, iface, err := c.d.SNMP.Lookup(exporterIP, uint(flow.InIf))
		if err != nil {
			if err != snmp.ErrCacheMiss {
				errLogger.Err(err).Str("exporter", exporterStr).Msg("unable to query SNMP cache")
			}
			c.metrics.flowsErrors.WithLabelValues(exporterStr, err.Error()).Inc()
			skip = true
		} else {
			flow.ExporterName = exporterName
			flow.InIfName = iface.Name
			flow.InIfDescription = iface.Description
			flow.InIfSpeed = uint32(iface.Speed)
		}
	}

	if flow.OutIf != 0 {
		exporterName, iface, err := c.d.SNMP.Lookup(exporterIP, uint(flow.OutIf))
		if err != nil {
			// Only register a cache miss if we don't have one.
			// TODO: maybe we could do one SNMP query for both interfaces.
			if !skip {
				if err != snmp.ErrCacheMiss {
					errLogger.Err(err).Str("exporter", exporterStr).Msg("unable to query SNMP cache")
				}
				c.metrics.flowsErrors.WithLabelValues(exporterStr, err.Error()).Inc()
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
		c.metrics.flowsErrors.WithLabelValues(exporterStr, "input and output interfaces missing").Inc()
		skip = true
	}

	if samplingRate, ok := c.config.OverrideSamplingRate.Lookup(exporterIP); ok && samplingRate > 0 {
		flow.SamplingRate = uint64(samplingRate)
	}
	if flow.SamplingRate == 0 {
		if samplingRate, ok := c.config.DefaultSamplingRate.Lookup(exporterIP); ok && samplingRate > 0 {
			flow.SamplingRate = uint64(samplingRate)
		} else {
			c.metrics.flowsErrors.WithLabelValues(exporterStr, "sampling rate missing").Inc()
			skip = true
		}
	}

	if skip {
		return
	}

	// Classification
	c.classifyExporter(exporterStr, flow)
	c.classifyInterface(exporterStr, flow,
		&flow.OutIfName, &flow.OutIfDescription, flow.OutIfSpeed,
		&flow.OutIfConnectivity, &flow.OutIfProvider, &flow.OutIfBoundary)
	c.classifyInterface(exporterStr, flow,
		&flow.InIfName, &flow.InIfDescription, flow.InIfSpeed,
		&flow.InIfConnectivity, &flow.InIfProvider, &flow.InIfBoundary)

	sourceBMP := c.d.BMP.Lookup(net.IP(flow.SrcAddr), nil)
	destBMP := c.d.BMP.Lookup(net.IP(flow.DstAddr), net.IP(flow.NextHop))
	flow.SrcAS = c.getASNumber(net.IP(flow.SrcAddr), flow.SrcAS, sourceBMP.ASN)
	flow.DstAS = c.getASNumber(net.IP(flow.DstAddr), flow.DstAS, destBMP.ASN)
	flow.SrcCountry = c.d.GeoIP.LookupCountry(net.IP(flow.SrcAddr))
	flow.DstCountry = c.d.GeoIP.LookupCountry(net.IP(flow.DstAddr))

	flow.DstCommunities = destBMP.Communities
	flow.DstASPath = destBMP.ASPath
	if len(destBMP.LargeCommunities) > 0 {
		flow.DstLargeCommunities = &decoder.FlowMessage_LargeCommunities{
			ASN:        make([]uint32, len(destBMP.LargeCommunities)),
			LocalData1: make([]uint32, len(destBMP.LargeCommunities)),
			LocalData2: make([]uint32, len(destBMP.LargeCommunities)),
		}
		for i := 0; i < len(destBMP.LargeCommunities); i++ {
			flow.DstLargeCommunities.ASN[i] = destBMP.LargeCommunities[i].ASN
			flow.DstLargeCommunities.LocalData1[i] = destBMP.LargeCommunities[i].LocalData1
			flow.DstLargeCommunities.LocalData2[i] = destBMP.LargeCommunities[i].LocalData2
		}
	}

	return
}

// getASNumber retrieves the AS number for a flow, depending on user preferences.
func (c *Component) getASNumber(flowAddr net.IP, flowAS, bmpAS uint32) (asn uint32) {
	for _, provider := range c.config.ASNProviders {
		if asn != 0 {
			break
		}
		switch provider {
		case ProviderGeoIP:
			asn = c.d.GeoIP.LookupASN(flowAddr)
		case ProviderFlow:
			asn = flowAS
		case ProviderFlowExceptPrivate:
			asn = flowAS
			if isPrivateAS(asn) {
				asn = 0
			}
		case ProviderBMP:
			asn = bmpAS
		case ProviderBMPExceptPrivate:
			asn = bmpAS
			if isPrivateAS(asn) {
				asn = 0
			}
		}
	}
	return asn
}

func (c *Component) classifyExporter(ip string, flow *flow.Message) {
	if len(c.config.ExporterClassifiers) == 0 {
		return
	}
	name := flow.ExporterName
	si := exporterInfo{IP: ip, Name: name}
	if classification, ok := c.classifierExporterCache.Get(si); ok {
		flow.ExporterGroup = classification.Group
		flow.ExporterRole = classification.Role
		flow.ExporterSite = classification.Site
		flow.ExporterRegion = classification.Region
		flow.ExporterTenant = classification.Tenant
		return
	}

	var classification exporterClassification
	for idx, rule := range c.config.ExporterClassifiers {
		if err := rule.exec(si, &classification); err != nil {
			c.classifierErrLogger.Err(err).
				Str("type", "exporter").
				Int("index", idx).
				Str("exporter", name).
				Msg("error executing classifier")
			c.metrics.classifierErrors.WithLabelValues("exporter", strconv.Itoa(idx)).Inc()
			c.classifierExporterCache.Set(si, classification)
			return
		}
		if classification.Group == "" || classification.Role == "" || classification.Site == "" || classification.Region == "" || classification.Tenant == "" {
			continue
		}
		break
	}
	c.classifierExporterCache.Set(si, classification)
	flow.ExporterGroup = classification.Group
	flow.ExporterRole = classification.Role
	flow.ExporterSite = classification.Site
	flow.ExporterRegion = classification.Region
	flow.ExporterTenant = classification.Tenant
}

func (c *Component) classifyInterface(ip string, fl *flow.Message,
	ifName, ifDescription *string, ifSpeed uint32,
	connectivity, provider *string, boundary *decoder.FlowMessage_Boundary) {
	if len(c.config.InterfaceClassifiers) == 0 {
		return
	}
	si := exporterInfo{IP: ip, Name: fl.ExporterName}
	ii := interfaceInfo{Name: *ifName, Description: *ifDescription, Speed: ifSpeed, VLAN: fmt.Sprintf("%d", fl.VlanID)}
	key := exporterAndInterfaceInfo{
		Exporter:  si,
		Interface: ii,
	}
	if classification, ok := c.classifierInterfaceCache.Get(key); ok {
		*connectivity = classification.Connectivity
		*provider = classification.Provider
		*boundary = convertBoundaryToProto(classification.Boundary)
		return
	}

	var classification interfaceClassification
	for idx, rule := range c.config.InterfaceClassifiers {
		err := rule.exec(si, &ii, &classification)
		if err != nil {
			c.classifierErrLogger.Err(err).
				Str("type", "interface").
				Int("index", idx).
				Str("exporter", fl.ExporterName).
				Str("interface", *ifName).
				Msg("error executing classifier")
			c.metrics.classifierErrors.WithLabelValues("interface", strconv.Itoa(idx)).Inc()
			c.classifierInterfaceCache.Set(key, classification)
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
	c.classifierInterfaceCache.Set(key, classification)
	*connectivity = classification.Connectivity
	*provider = classification.Provider
	*boundary = convertBoundaryToProto(classification.Boundary)
	*ifName = ii.Name
	*ifDescription = ii.Description
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

func isPrivateAS(as uint32) bool {
	// See https://www.iana.org/assignments/iana-as-numbers-special-registry/iana-as-numbers-special-registry.xhtml
	if as == 0 || as == 23456 {
		return true
	}
	if 64496 <= as && as <= 65551 || 4_200_000_000 <= as && as <= 4_294_967_295 {
		return true
	}
	return false
}
