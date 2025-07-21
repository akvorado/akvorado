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
func (w *worker) enrichFlow(exporterIP netip.Addr, exporterStr string) (skip bool) {
	var flowExporterName string
	var flowInIfName, flowInIfDescription, flowOutIfName, flowOutIfDescription string
	var flowInIfSpeed, flowOutIfSpeed, flowInIfIndex, flowOutIfIndex uint32
	var flowInIfVlan, flowOutIfVlan uint16

	t := time.Now() // only call it once
	expClassification := exporterClassification{}
	inIfClassification := interfaceClassification{}
	outIfClassification := interfaceClassification{}

	flow := w.bf
	c := w.c

	if flow.InIf != 0 {
		answer := c.d.Metadata.Lookup(t, exporterIP, uint(flow.InIf))
		if !answer.Found {
			c.metrics.flowsErrors.WithLabelValues(exporterStr, "SNMP cache miss").Inc()
			skip = true
		} else {
			flowExporterName = answer.Exporter.Name
			expClassification.Region = answer.Exporter.Region
			expClassification.Role = answer.Exporter.Role
			expClassification.Tenant = answer.Exporter.Tenant
			expClassification.Site = answer.Exporter.Site
			expClassification.Group = answer.Exporter.Group
			flowInIfIndex = flow.InIf
			flowInIfName = answer.Interface.Name
			flowInIfDescription = answer.Interface.Description
			flowInIfSpeed = uint32(answer.Interface.Speed)
			inIfClassification.Provider = answer.Interface.Provider
			inIfClassification.Connectivity = answer.Interface.Connectivity
			inIfClassification.Boundary = answer.Interface.Boundary
			flowInIfVlan = flow.SrcVlan
		}
	}

	if flow.OutIf != 0 {
		answer := c.d.Metadata.Lookup(t, exporterIP, uint(flow.OutIf))
		if !answer.Found {
			// Only register a cache miss if we don't have one.
			// TODO: maybe we could do one SNMP query for both interfaces.
			if !skip {
				c.metrics.flowsErrors.WithLabelValues(exporterStr, "SNMP cache miss").Inc()
				skip = true
			}
		} else {
			flowExporterName = answer.Exporter.Name
			expClassification.Region = answer.Exporter.Region
			expClassification.Role = answer.Exporter.Role
			expClassification.Tenant = answer.Exporter.Tenant
			expClassification.Site = answer.Exporter.Site
			expClassification.Group = answer.Exporter.Group
			flowOutIfIndex = flow.OutIf
			flowOutIfName = answer.Interface.Name
			flowOutIfDescription = answer.Interface.Description
			flowOutIfSpeed = uint32(answer.Interface.Speed)
			outIfClassification.Provider = answer.Interface.Provider
			outIfClassification.Connectivity = answer.Interface.Connectivity
			outIfClassification.Boundary = answer.Interface.Boundary
			flowOutIfVlan = flow.DstVlan
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
	flow.SrcAS = c.getASNumber(flow.SrcAS, sourceRouting.ASN)
	flow.DstAS = c.getASNumber(flow.DstAS, destRouting.ASN)
	flow.AppendArrayUInt32(schema.ColumnDstCommunities, destRouting.Communities)
	flow.AppendArrayUInt32(schema.ColumnDstASPath, destRouting.ASPath)
	if len(destRouting.LargeCommunities) > 0 {
		communities := make([]schema.UInt128, len(destRouting.LargeCommunities))
		for i, comm := range destRouting.LargeCommunities {
			communities[i] = schema.UInt128{
				High: uint64(comm.ASN),
				Low:  (uint64(comm.LocalData1) << 32) + uint64(comm.LocalData2),
			}
		}
		flow.AppendArrayUInt128(schema.ColumnDstLargeCommunities, communities)
	}

	flow.AppendString(schema.ColumnExporterName, flowExporterName)
	flow.AppendUint(schema.ColumnInIfSpeed, uint64(flowInIfSpeed))
	flow.AppendUint(schema.ColumnOutIfSpeed, uint64(flowOutIfSpeed))

	return
}

// getASNumber retrieves the AS number for a flow, depending on user preferences.
func (c *Component) getASNumber(flowAS, bmpAS uint32) (asn uint32) {
	for _, provider := range c.config.ASNProviders {
		if asn != 0 {
			break
		}
		switch provider {
		case ASNProviderGeoIP:
			// This is a shortcut
			return 0
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
	flow.AppendString(schema.ColumnExporterGroup, classification.Group)
	flow.AppendString(schema.ColumnExporterRole, classification.Role)
	flow.AppendString(schema.ColumnExporterSite, classification.Site)
	flow.AppendString(schema.ColumnExporterRegion, classification.Region)
	flow.AppendString(schema.ColumnExporterTenant, classification.Tenant)
	return true
}

func (c *Component) classifyExporter(t time.Time, ip string, name string, flow *schema.FlowMessage, classification exporterClassification) bool {
	// we already have the info provided by the metadata component
	if (classification != exporterClassification{}) {
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
		flow.AppendString(schema.ColumnInIfName, classification.Name)
		flow.AppendString(schema.ColumnInIfDescription, classification.Description)
		flow.AppendString(schema.ColumnInIfConnectivity, classification.Connectivity)
		flow.AppendString(schema.ColumnInIfProvider, classification.Provider)
		flow.AppendUint(schema.ColumnInIfBoundary, uint64(classification.Boundary))
	} else {
		flow.AppendString(schema.ColumnOutIfName, classification.Name)
		flow.AppendString(schema.ColumnOutIfDescription, classification.Description)
		flow.AppendString(schema.ColumnOutIfConnectivity, classification.Connectivity)
		flow.AppendString(schema.ColumnOutIfProvider, classification.Provider)
		flow.AppendUint(schema.ColumnOutIfBoundary, uint64(classification.Boundary))
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
	if (classification != interfaceClassification{}) {
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
