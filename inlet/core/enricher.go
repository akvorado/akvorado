// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"net/netip"
	"strconv"
	"time"

	"akvorado/common/schema"
	"akvorado/inlet/bmp"
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

	if flow.InIf != 0 {
		exporterName, iface, ok := c.d.Metadata.Lookup(t, exporterIP, uint(flow.InIf))
		if !ok {
			c.metrics.flowsErrors.WithLabelValues(exporterStr, "SNMP cache miss").Inc()
			skip = true
		} else {
			flowExporterName = exporterName
			flowInIfIndex = flow.InIf
			flowInIfName = iface.Name
			flowInIfDescription = iface.Description
			flowInIfSpeed = uint32(iface.Speed)
			flowInIfVlan = flow.SrcVlan
		}
	}

	if flow.OutIf != 0 {
		exporterName, iface, ok := c.d.Metadata.Lookup(t, exporterIP, uint(flow.OutIf))
		if !ok {
			// Only register a cache miss if we don't have one.
			// TODO: maybe we could do one SNMP query for both interfaces.
			if !skip {
				c.metrics.flowsErrors.WithLabelValues(exporterStr, "SNMP cache miss").Inc()
				skip = true
			}
		} else {
			flowExporterName = exporterName
			flowOutIfIndex = flow.OutIf
			flowOutIfName = iface.Name
			flowOutIfDescription = iface.Description
			flowOutIfSpeed = uint32(iface.Speed)
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
	if !c.classifyExporter(t, exporterStr, flowExporterName, flow) ||
		!c.classifyInterface(t, exporterStr, flowExporterName, flow,
			flowOutIfIndex, flowOutIfName, flowOutIfDescription, flowOutIfSpeed, flowOutIfVlan,
			false) ||
		!c.classifyInterface(t, exporterStr, flowExporterName, flow,
			flowInIfIndex, flowInIfName, flowInIfDescription, flowInIfSpeed, flowInIfVlan,
			true) {
		// Flow is rejected
		return true
	}

	// by default, we use the internal BMP. However, if an bioris configuration is available, we swap this to bioris.
	var (
		sourceBMP bmp.LookupResult
		destBMP   bmp.LookupResult
		err       error
	)
	if c.config.RISProvider == RISProviderBioRis {
		sourceBMP, err = c.d.BioRIS.Lookup(flow.SrcAddr, flow.ExporterAddress, netip.IPv4Unspecified())
		if err != nil {
			c.metrics.flowsLookupFailed.WithLabelValues(exporterStr).Inc()
			c.r.Logger.Warn().Err(err).Msg("failed to lookup src ip in BioRIS")
			// flow is rejected
			return true
		}
		destBMP, err = c.d.BioRIS.Lookup(flow.DstAddr, flow.ExporterAddress, flow.NextHop)
		if err != nil {
			c.metrics.flowsLookupFailed.WithLabelValues(exporterStr).Inc()
			c.r.Logger.Warn().Err(err).Msg("failed to lookup dst ip in BioRIS")
			// flow is rejected
			return true
		}
	} else {
		sourceBMP = c.d.BMP.Lookup(flow.SrcAddr, netip.IPv4Unspecified())
		destBMP = c.d.BMP.Lookup(flow.DstAddr, flow.NextHop)
	}
	// set prefix len according to user config
	flow.SrcNetMask = c.getNetMask(flow.SrcNetMask, sourceBMP.NetMask)
	flow.DstNetMask = c.getNetMask(flow.DstNetMask, destBMP.NetMask)

	// set asns according to user config
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

// getNetMask retrieves the prefix length for a flow, depending on user preferences.
func (c *Component) getNetMask(flowMask, bmpMask uint8) (mask uint8) {
	for _, provider := range c.config.NetProviders {
		if mask != 0 {
			break
		}
		switch provider {
		case NetProviderFlow:
			mask = flowMask
		case NetProviderBMP:
			mask = bmpMask
		}
	}
	return mask
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

func (c *Component) classifyExporter(t time.Time, ip string, name string, flow *schema.FlowMessage) bool {
	if len(c.config.ExporterClassifiers) == 0 {
		return true
	}
	si := exporterInfo{IP: ip, Name: name}
	if classification, ok := c.classifierExporterCache.Get(t, si); ok {
		return c.writeExporter(flow, classification)
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

func (c *Component) classifyInterface(t time.Time, ip string, exporterName string, fl *schema.FlowMessage, ifIndex uint32, ifName, ifDescription string, ifSpeed uint32, ifVlan uint16, directionIn bool) bool {
	if len(c.config.InterfaceClassifiers) == 0 {
		c.writeInterface(fl, interfaceClassification{
			Name:        ifName,
			Description: ifDescription,
		}, directionIn)
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
			break
		}
		if classification.Connectivity == "" || classification.Provider == "" {
			continue
		}
		if classification.Boundary == undefinedBoundary {
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
