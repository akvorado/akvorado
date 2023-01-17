// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"net/netip"
	"strconv"
	"time"

	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/snmp"
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
			flowExporterName = exporterName
			flowInIfName = iface.Name
			flowInIfDescription = iface.Description
			flowInIfSpeed = uint32(iface.Speed)
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
	c.classifyExporter(exporterStr, flowExporterName, flow)
	c.classifyInterface(exporterStr, flowExporterName, flow,
		flowOutIfName, flowOutIfDescription, flowOutIfSpeed,
		false)
	c.classifyInterface(exporterStr, flowExporterName, flow,
		flowInIfName, flowInIfDescription, flowInIfSpeed,
		true)

	sourceBMP := c.d.BMP.Lookup(flow.SrcAddr, netip.Addr{})
	destBMP := c.d.BMP.Lookup(flow.DstAddr, flow.NextHop)
	flow.SrcAS = c.getASNumber(flow.SrcAddr, flow.SrcAS, sourceBMP.ASN)
	flow.DstAS = c.getASNumber(flow.DstAddr, flow.DstAS, destBMP.ASN)
	schema.Flows.ProtobufAppendBytes(flow, schema.ColumnSrcCountry, []byte(c.d.GeoIP.LookupCountry(flow.SrcAddr)))
	schema.Flows.ProtobufAppendBytes(flow, schema.ColumnDstCountry, []byte(c.d.GeoIP.LookupCountry(flow.DstAddr)))
	for _, comm := range destBMP.Communities {
		schema.Flows.ProtobufAppendVarint(flow, schema.ColumnDstCommunities, uint64(comm))
	}
	for _, asn := range destBMP.ASPath {
		schema.Flows.ProtobufAppendVarint(flow, schema.ColumnDstASPath, uint64(asn))
	}
	for _, comm := range destBMP.LargeCommunities {
		schema.Flows.ProtobufAppendVarintForce(flow,
			schema.ColumnDstLargeCommunitiesASN, uint64(comm.ASN))
		schema.Flows.ProtobufAppendVarintForce(flow,
			schema.ColumnDstLargeCommunitiesLocalData1, uint64(comm.LocalData1))
		schema.Flows.ProtobufAppendVarintForce(flow,
			schema.ColumnDstLargeCommunitiesLocalData2, uint64(comm.LocalData2))
	}

	schema.Flows.ProtobufAppendBytes(flow, schema.ColumnExporterName, []byte(flowExporterName))
	schema.Flows.ProtobufAppendBytes(flow, schema.ColumnInIfName, []byte(flowInIfName))
	schema.Flows.ProtobufAppendBytes(flow, schema.ColumnInIfDescription, []byte(flowInIfDescription))
	schema.Flows.ProtobufAppendBytes(flow, schema.ColumnOutIfName, []byte(flowOutIfName))
	schema.Flows.ProtobufAppendBytes(flow, schema.ColumnOutIfDescription, []byte(flowOutIfDescription))
	schema.Flows.ProtobufAppendVarint(flow, schema.ColumnInIfSpeed, uint64(flowInIfSpeed))
	schema.Flows.ProtobufAppendVarint(flow, schema.ColumnOutIfSpeed, uint64(flowOutIfSpeed))

	return
}

// getASNumber retrieves the AS number for a flow, depending on user preferences.
func (c *Component) getASNumber(flowAddr netip.Addr, flowAS, bmpAS uint32) (asn uint32) {
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

func writeExporter(flow *schema.FlowMessage, classification exporterClassification) {
	schema.Flows.ProtobufAppendBytes(flow, schema.ColumnExporterGroup, []byte(classification.Group))
	schema.Flows.ProtobufAppendBytes(flow, schema.ColumnExporterRole, []byte(classification.Role))
	schema.Flows.ProtobufAppendBytes(flow, schema.ColumnExporterSite, []byte(classification.Site))
	schema.Flows.ProtobufAppendBytes(flow, schema.ColumnExporterRegion, []byte(classification.Region))
	schema.Flows.ProtobufAppendBytes(flow, schema.ColumnExporterTenant, []byte(classification.Tenant))
}

func (c *Component) classifyExporter(ip string, name string, flow *schema.FlowMessage) {
	if len(c.config.ExporterClassifiers) == 0 {
		return
	}
	si := exporterInfo{IP: ip, Name: name}
	if classification, ok := c.classifierExporterCache.Get(si); ok {
		writeExporter(flow, classification)
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
	writeExporter(flow, classification)
}

func writeInterface(flow *schema.FlowMessage, classification interfaceClassification, directionIn bool) {
	if directionIn {
		schema.Flows.ProtobufAppendBytes(flow, schema.ColumnInIfConnectivity, []byte(classification.Connectivity))
		schema.Flows.ProtobufAppendBytes(flow, schema.ColumnInIfProvider, []byte(classification.Provider))
		schema.Flows.ProtobufAppendVarint(flow, schema.ColumnInIfBoundary, uint64(classification.Boundary))
	} else {
		schema.Flows.ProtobufAppendBytes(flow, schema.ColumnOutIfConnectivity, []byte(classification.Connectivity))
		schema.Flows.ProtobufAppendBytes(flow, schema.ColumnOutIfProvider, []byte(classification.Provider))
		schema.Flows.ProtobufAppendVarint(flow, schema.ColumnOutIfBoundary, uint64(classification.Boundary))
	}
}

func (c *Component) classifyInterface(ip string, exporterName string, fl *schema.FlowMessage, ifName, ifDescription string, ifSpeed uint32, directionIn bool) {
	if len(c.config.InterfaceClassifiers) == 0 {
		return
	}
	si := exporterInfo{IP: ip, Name: exporterName}
	ii := interfaceInfo{Name: ifName, Description: ifDescription, Speed: ifSpeed}
	key := exporterAndInterfaceInfo{
		Exporter:  si,
		Interface: ii,
	}
	if classification, ok := c.classifierInterfaceCache.Get(key); ok {
		writeInterface(fl, classification, directionIn)
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
	writeInterface(fl, classification, directionIn)
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
