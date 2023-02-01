// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
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
	var flowInIfSpeed, flowOutIfSpeed uint32

	t := time.Now() // only call it once
	if flow.TimeReceived == 0 {
		flow.TimeReceived = uint64(t.UTC().Unix())
	}

	if flow.InIf != 0 {
		exporterName, iface, ok := c.d.SNMP.Lookup(t, exporterIP, uint(flow.InIf))
		if !ok {
			c.metrics.flowsErrors.WithLabelValues(exporterStr, "SNMP cache miss").Inc()
			skip = true
		} else {
			flowExporterName = exporterName
			flowInIfName = iface.Name
			flowInIfDescription = iface.Description
			flowInIfSpeed = uint32(iface.Speed)
		}
	}

	if flow.OutIf != 0 {
		exporterName, iface, ok := c.d.SNMP.Lookup(t, exporterIP, uint(flow.OutIf))
		if !ok {
			// Only register a cache miss if we don't have one.
			// TODO: maybe we could do one SNMP query for both interfaces.
			if !skip {
				c.metrics.flowsErrors.WithLabelValues(exporterStr, "SNMP cache miss").Inc()
				skip = true
			}
		} else {
			flowExporterName = exporterName
			flowOutIfName = iface.Name
			flowOutIfDescription = iface.Description
			flowOutIfSpeed = uint32(iface.Speed)
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
	c.classifyExporter(t, exporterStr, flowExporterName, flow)
	c.classifyInterface(t, exporterStr, flowExporterName, flow,
		flowOutIfName, flowOutIfDescription, flowOutIfSpeed,
		false)
	c.classifyInterface(t, exporterStr, flowExporterName, flow,
		flowInIfName, flowInIfDescription, flowInIfSpeed,
		true)

	sourceBMP := c.d.BMP.Lookup(flow.SrcAddr, netip.Addr{})
	destBMP := c.d.BMP.Lookup(flow.DstAddr, flow.NextHop)
	flow.SrcAS = c.getASNumber(flow.SrcAddr, flow.SrcAS, sourceBMP.ASN)
	flow.DstAS = c.getASNumber(flow.DstAddr, flow.DstAS, destBMP.ASN)
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnSrcCountry, []byte(c.d.GeoIP.LookupCountry(flow.SrcAddr)))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnDstCountry, []byte(c.d.GeoIP.LookupCountry(flow.DstAddr)))
	for _, comm := range destBMP.Communities {
		c.d.Schema.ProtobufAppendVarint(flow, schema.ColumnDstCommunities, uint64(comm))
	}
	if !flow.GotASPath {
		for _, asn := range destBMP.ASPath {
			c.d.Schema.ProtobufAppendVarint(flow, schema.ColumnDstASPath, uint64(asn))
		}
	}
	for _, comm := range destBMP.LargeCommunities {
		c.d.Schema.ProtobufAppendVarintForce(flow,
			schema.ColumnDstLargeCommunitiesASN, uint64(comm.ASN))
		c.d.Schema.ProtobufAppendVarintForce(flow,
			schema.ColumnDstLargeCommunitiesLocalData1, uint64(comm.LocalData1))
		c.d.Schema.ProtobufAppendVarintForce(flow,
			schema.ColumnDstLargeCommunitiesLocalData2, uint64(comm.LocalData2))
	}

	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnExporterName, []byte(flowExporterName))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnInIfName, []byte(flowInIfName))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnInIfDescription, []byte(flowInIfDescription))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnOutIfName, []byte(flowOutIfName))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnOutIfDescription, []byte(flowOutIfDescription))
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
		case ASNProviderBMP:
			asn = bmpAS
		case ASNProviderBMPExceptPrivate:
			asn = bmpAS
			if isPrivateAS(asn) {
				asn = 0
			}
		}
	}
	return asn
}

func (c *Component) writeExporter(flow *schema.FlowMessage, classification exporterClassification) {
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnExporterGroup, []byte(classification.Group))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnExporterRole, []byte(classification.Role))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnExporterSite, []byte(classification.Site))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnExporterRegion, []byte(classification.Region))
	c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnExporterTenant, []byte(classification.Tenant))
}

func (c *Component) classifyExporter(t time.Time, ip string, name string, flow *schema.FlowMessage) {
	if len(c.config.ExporterClassifiers) == 0 {
		return
	}
	si := exporterInfo{IP: ip, Name: name}
	if classification, ok := c.classifierExporterCache.Get(t, si); ok {
		c.writeExporter(flow, classification)
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
			c.classifierExporterCache.Put(t, si, classification)
			return
		}
		if classification.Group == "" || classification.Role == "" || classification.Site == "" || classification.Region == "" || classification.Tenant == "" {
			continue
		}
		break
	}
	c.classifierExporterCache.Put(t, si, classification)
	c.writeExporter(flow, classification)
}

func (c *Component) writeInterface(flow *schema.FlowMessage, classification interfaceClassification, directionIn bool) {
	if directionIn {
		c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnInIfConnectivity, []byte(classification.Connectivity))
		c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnInIfProvider, []byte(classification.Provider))
		c.d.Schema.ProtobufAppendVarint(flow, schema.ColumnInIfBoundary, uint64(classification.Boundary))
	} else {
		c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnOutIfConnectivity, []byte(classification.Connectivity))
		c.d.Schema.ProtobufAppendBytes(flow, schema.ColumnOutIfProvider, []byte(classification.Provider))
		c.d.Schema.ProtobufAppendVarint(flow, schema.ColumnOutIfBoundary, uint64(classification.Boundary))
	}
}

func (c *Component) classifyInterface(t time.Time, ip string, exporterName string, fl *schema.FlowMessage, ifName, ifDescription string, ifSpeed uint32, directionIn bool) {
	if len(c.config.InterfaceClassifiers) == 0 {
		return
	}
	si := exporterInfo{IP: ip, Name: exporterName}
	ii := interfaceInfo{Name: ifName, Description: ifDescription, Speed: ifSpeed}
	key := exporterAndInterfaceInfo{
		Exporter:  si,
		Interface: ii,
	}
	if classification, ok := c.classifierInterfaceCache.Get(t, key); ok {
		c.writeInterface(fl, classification, directionIn)
		return
	}

	var classification interfaceClassification
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
			c.classifierInterfaceCache.Put(t, key, classification)
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
	c.classifierInterfaceCache.Put(t, key, classification)
	c.writeInterface(fl, classification, directionIn)
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
