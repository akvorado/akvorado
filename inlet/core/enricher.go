// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"context"
	"net/netip"
	"strconv"
	"time"

	"akvorado/common/schema"
)

// exporterAndInterfaceInfo aggregates both exporter info and interface info
type exporterAndInterfaceInfo struct {
	Exporter  exporterInfo
	Interface interfaceInfo
}

// enrichFlow adds more data to a flow.
func (c *Component) enrichFlow(exporterIP netip.Addr, exporterStr string, flow *schema.FlowMessage) (skip bool) {
	var flowExporterName string
	var flowInIfName, flowInIfDescription, flowOutIfName, flowOutIfDescription string
	var flowInIfSpeed, flowOutIfSpeed, flowInIfIndex, flowOutIfIndex uint32
	var flowInIfVlan, flowOutIfVlan uint16

	t := time.Now() // only call it once
	expClassification := exporterClassification{}
	inIfClassification := interfaceClassification{}
	outIfClassification := interfaceClassification{}

	if flow.InIf != 0 {
		answer, ok := c.d.Metadata.Lookup(t, exporterIP, uint(flow.InIf))
		if !ok {
			c.metrics.flowsErrors.WithLabelValues(exporterStr, "SNMP cache miss").Inc()
			skip = true
		} else {
			flowExporterName = answer.ExporterName
			expClassification.Region = answer.ExporterRegion
			expClassification.Role = answer.ExporterRole
			expClassification.Tenant = answer.ExporterTenant
			expClassification.Site = answer.ExporterSite
			expClassification.Group = answer.ExporterGroup
			flowInIfIndex = flow.InIf
			flowInIfName = answer.InterfaceName
			flowInIfDescription = answer.InterfaceDescription
			flowInIfSpeed = uint32(answer.InterfaceSpeed)
			inIfClassification.Provider = answer.InterfaceProvider
			inIfClassification.Connectivity = answer.InterfaceConnectivity
			inIfClassification.Boundary = answer.InterfaceBoundary
			flowInIfVlan = flow.SrcVlan
		}
	}

	if flow.OutIf != 0 {
		answer, ok := c.d.Metadata.Lookup(t, exporterIP, uint(flow.OutIf))
		if !ok {
			// Only register a cache miss if we don't have one.
			// TODO: maybe we could do one SNMP query for both interfaces.
			if !skip {
				c.metrics.flowsErrors.WithLabelValues(exporterStr, "SNMP cache miss").Inc()
				skip = true
			}
		} else {
			flowExporterName = answer.ExporterName
			expClassification.Region = answer.ExporterRegion
			expClassification.Role = answer.ExporterRole
			expClassification.Tenant = answer.ExporterTenant
			expClassification.Site = answer.ExporterSite
			expClassification.Group = answer.ExporterGroup
			flowOutIfIndex = flow.OutIf
			flowOutIfName = answer.InterfaceName
			flowOutIfDescription = answer.InterfaceDescription
			flowOutIfSpeed = uint32(answer.InterfaceSpeed)
			outIfClassification.Provider = answer.InterfaceProvider
			outIfClassification.Connectivity = answer.InterfaceConnectivity
			outIfClassification.Boundary = answer.InterfaceBoundary
			flowOutIfVlan = flow.DstVlan
		}
	}

	// We need at least one of them.
	if flow.OutIf == 0 && flow.InIf == 0 {
		c.metrics.flowsErrors.WithLabelValues(exporterStr, "input and output interfaces missing").Inc()
		skip = true
	}

	if samplingRate, ok := c.config.OverrideSamplingRate.Lookup(exporterIP); ok && samplingRate > 0 {
		flow.SamplingRate = uint32(samplingRate)
	}
	if flow.SamplingRate == 0 {
		if samplingRate, ok := c.config.DefaultSamplingRate.Lookup(exporterIP); ok && samplingRate > 0 {
			flow.SamplingRate = uint32(samplingRate)
		} else {
			c.metrics.flowsErrors.WithLabelValues(exporterStr, "sampling rate missing").Inc()
			skip = true
		}
	}

	if skip {
		return
	}

	// Classification
	if !c.classifyExporter(t, exporterStr, flowExporterName, flow, expClassification) ||
		!c.classifyInterface(t, exporterStr, flowExporterName, flow,
			flowOutIfIndex, flowOutIfName, flowOutIfDescription, flowOutIfSpeed, flowOutIfVlan, outIfClassification,
			false) ||
		!c.classifyInterface(t, exporterStr, flowExporterName, flow,
			flowInIfIndex, flowInIfName, flowInIfDescription, flowInIfSpeed, flowInIfVlan, inIfClassification,
			true) {
		// Flow is rejected
		return true
	}

	ctx := c.t.Context(context.Background())
	sourceRouting := c.d.Routing.Lookup(ctx, flow.SrcAddr, netip.Addr{}, flow.ExporterAddress)
	destRouting := c.d.Routing.Lookup(ctx, flow.DstAddr, flow.NextHop, flow.ExporterAddress)

	// set prefix len according to user config
	flow.SrcNetMask = c.getNetMask(flow.SrcNetMask, sourceRouting.NetMask)
	flow.DstNetMask = c.getNetMask(flow.DstNetMask, destRouting.NetMask)

	// set next hop according to user config
	flow.NextHop = c.getNextHop(flow.NextHop, destRouting.NextHop)

	// set asns according to user config
	flow.SrcAS = c.getASNumber(flow.SrcAddr, flow.SrcAS, sourceRouting.ASN)
	flow.DstAS = c.getASNumber(flow.DstAddr, flow.DstAS, destRouting.ASN)
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnSrcCountry, []byte(c.d.GeoIP.LookupCountry(flow.SrcAddr)))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnDstCountry, []byte(c.d.GeoIP.LookupCountry(flow.DstAddr)))
	for _, comm := range destRouting.Communities {
		c.d.Schema.ProtobufAppendVarint(flow, schema.ColumnDstCommunities, uint64(comm))
	}
	if !flow.GotASPath {
		for _, asn := range destRouting.ASPath {
			c.d.Schema.ProtobufAppendVarint(flow, schema.ColumnDstASPath, uint64(asn))
		}
	}
	for _, comm := range destRouting.LargeCommunities {
		c.d.Schema.ProtobufAppendVarintForce(flow,
			schema.ColumnDstLargeCommunitiesASN, uint64(comm.ASN))
		c.d.Schema.ProtobufAppendVarintForce(flow,
			schema.ColumnDstLargeCommunitiesLocalData1, uint64(comm.LocalData1))
		c.d.Schema.ProtobufAppendVarintForce(flow,
			schema.ColumnDstLargeCommunitiesLocalData2, uint64(comm.LocalData2))
	}

	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnExporterName, []byte(flowExporterName))
	c.d.Schema.ProtobufAppendVarint(flow, schema.ColumnInIfSpeed, uint64(flowInIfSpeed))
	c.d.Schema.ProtobufAppendVarint(flow, schema.ColumnOutIfSpeed, uint64(flowOutIfSpeed))

	return
}

// getASNumber retrieves the AS number for a flow, depending on user preferences.
func (c *Component) getASNumber(flowAddr netip.Addr, flowAS, bmpAS uint32) (asn uint32) {
	for _, provider := range c.config.ASNProviders {
		if asn != 0 {
			break
		}
		switch provider {
		case ASNProviderGeoIP:
			asn = c.d.GeoIP.LookupASN(flowAddr)
		case ASNProviderFlow:
			asn = flowAS
		case ASNProviderFlowExceptPrivate:
			asn = flowAS
			if isPrivateAS(asn) {
				asn = 0
			}
		case ASNProviderRouting:
			asn = bmpAS
		case ASNProviderRoutingExceptPrivate:
			asn = bmpAS
			if isPrivateAS(asn) {
				asn = 0
			}
		}
	}
	return asn
}

// getNetMask retrieves the prefix length for a flow, depending on user preferences.
func (c *Component) getNetMask(flowMask, bmpMask uint8) (mask uint8) {
	for _, provider := range c.config.NetProviders {
		if mask != 0 {
			break
		}
		switch provider {
		case NetProviderFlow:
			mask = flowMask
		case NetProviderRouting:
			mask = bmpMask
		}
	}
	return mask
}

func (c *Component) getNextHop(flowNextHop netip.Addr, bmpNextHop netip.Addr) (nextHop netip.Addr) {
	nextHop = netip.IPv6Unspecified()
	for _, provider := range c.config.NetProviders {
		if !nextHop.IsUnspecified() {
			break
		}
		switch provider {
		case NetProviderFlow:
			nextHop = flowNextHop
		case NetProviderRouting:
			nextHop = bmpNextHop
		}
	}
	return nextHop
}

func (c *Component) writeExporter(flow *schema.FlowMessage, classification exporterClassification) bool {
	if classification.Reject {
		return false
	}
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnExporterGroup, []byte(classification.Group))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnExporterRole, []byte(classification.Role))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnExporterSite, []byte(classification.Site))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnExporterRegion, []byte(classification.Region))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnExporterTenant, []byte(classification.Tenant))
	return true
}

func (c *Component) classifyExporter(t time.Time, ip string, name string, flow *schema.FlowMessage, classification exporterClassification) bool {
	// we already have the info provided by the metadata component
	if classification.Group != "" || classification.Role != "" || classification.Site != "" || classification.Region != "" || classification.Tenant != "" {
		return c.writeExporter(flow, classification)
	}
	if len(c.config.ExporterClassifiers) == 0 {
		return true
	}
	si := exporterInfo{IP: ip, Name: name}
	if classification, ok := c.classifierExporterCache.Get(t, si); ok {
		return c.writeExporter(flow, classification)
	}

	for idx, rule := range c.config.ExporterClassifiers {
		if err := rule.exec(si, &classification); err != nil {
			c.classifierErrLogger.Err(err).
				Str("type", "exporter").
				Int("index", idx).
				Str("exporter", name).
				Msg("error executing classifier")
			c.metrics.classifierErrors.WithLabelValues("exporter", strconv.Itoa(idx)).Inc()
			break
		}
		if classification.Group == "" || classification.Role == "" || classification.Site == "" || classification.Region == "" || classification.Tenant == "" {
			continue
		}
		break
	}
	c.classifierExporterCache.Put(t, si, classification)
	return c.writeExporter(flow, classification)
}

func (c *Component) writeInterface(flow *schema.FlowMessage, classification interfaceClassification, directionIn bool) bool {
	if classification.Reject {
		return false
	}
	if directionIn {
		c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnInIfName, []byte(classification.Name))
		c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnInIfDescription, []byte(classification.Description))
		c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnInIfConnectivity, []byte(classification.Connectivity))
		c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnInIfProvider, []byte(classification.Provider))
		c.d.Schema.ProtobufAppendVarint(flow, schema.ColumnInIfBoundary, uint64(classification.Boundary))
	} else {
		c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnOutIfName, []byte(classification.Name))
		c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnOutIfDescription, []byte(classification.Description))
		c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnOutIfConnectivity, []byte(classification.Connectivity))
		c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnOutIfProvider, []byte(classification.Provider))
		c.d.Schema.ProtobufAppendVarint(flow, schema.ColumnOutIfBoundary, uint64(classification.Boundary))
	}
	return true
}

func (c *Component) classifyInterface(
	t time.Time,
	ip string,
	exporterName string,
	fl *schema.FlowMessage,
	ifIndex uint32,
	ifName,
	ifDescription string,
	ifSpeed uint32,
	ifVlan uint16,
	classification interfaceClassification,
	directionIn bool,
) bool {
	// we already have the info provided by the metadata component
	if classification.Provider != "" || classification.Connectivity != "" || classification.Boundary != schema.InterfaceBoundaryUndefined {
		classification.Name = ifName
		classification.Description = ifDescription
		return c.writeInterface(fl, classification, directionIn)
	}
	if len(c.config.InterfaceClassifiers) == 0 {
		classification.Name = ifName
		classification.Description = ifDescription
		c.writeInterface(fl, classification, directionIn)
		return true
	}
	si := exporterInfo{IP: ip, Name: exporterName}
	ii := interfaceInfo{
		Index:       ifIndex,
		Name:        ifName,
		Description: ifDescription,
		Speed:       ifSpeed,
		VLAN:        ifVlan,
	}
	key := exporterAndInterfaceInfo{
		Exporter:  si,
		Interface: ii,
	}
	if classification, ok := c.classifierInterfaceCache.Get(t, key); ok {
		return c.writeInterface(fl, classification, directionIn)
	}

	for idx, rule := range c.config.InterfaceClassifiers {
		err := rule.exec(si, ii, &classification)
		if err != nil {
			c.classifierErrLogger.Err(err).
				Str("type", "interface").
				Int("index", idx).
				Str("exporter", exporterName).
				Str("interface", ifName).
				Msg("error executing classifier")
			c.metrics.classifierErrors.WithLabelValues("interface", strconv.Itoa(idx)).Inc()
			break
		}
		if classification.Connectivity == "" || classification.Provider == "" {
			continue
		}
		if classification.Boundary == schema.InterfaceBoundaryUndefined {
			continue
		}
		break
	}
	if classification.Name == "" {
		classification.Name = ifName
	}
	if classification.Description == "" {
		classification.Description = ifDescription
	}
	c.classifierInterfaceCache.Put(t, key, classification)
	return c.writeInterface(fl, classification, directionIn)
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
