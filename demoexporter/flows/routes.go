// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"fmt"
	"maps"
	"net/netip"
	"slices"

	"akvorado/demoexporter/bmp"
	"akvorado/demoexporter/flowmodel"
	"akvorado/demoexporter/snmp"
)

// SNMPInterfaces returns the interfaces of the emulated exporter.
func (c *Component) SNMPInterfaces() map[int]snmp.InterfaceConfiguration {
	interfaces := map[int]snmp.InterfaceConfiguration{}
	for ifIndex, iface := range c.config.Exporters[c.config.Exporter].Interfaces {
		interfaces[ifIndex] = iface.InterfaceConfiguration
	}
	return interfaces
}

// BMPRoutes returns the BMP routes of the emulated exporter.
func (c *Component) BMPRoutes() []bmp.RouteConfiguration {
	return bmpRoutes(&c.config, c.mapping, c.config.Exporter, c.metadata)
}

// bmpRoutes rebuilds the BMP routes of one exporter from the model metadata and
// the mapping. The model does not store AS paths: they are made from the AS
// number of the interface carrying the traffic and the origin AS number of the
// prefix. When several interfaces carry the traffic of a prefix, the one with
// the highest weight is used.
func bmpRoutes(config *Configuration, mapping *compiledMapping, exporter string, meta *flowmodel.Metadata) []bmp.RouteConfiguration {
	// Routes sharing the same AS path and communities are grouped.
	groups := map[string]*bmp.RouteConfiguration{}
	add := func(prefix netip.Prefix, path []uint32, communities []bmp.Community) {
		key := fmt.Sprintf("%v %v", path, communities)
		route, ok := groups[key]
		if !ok {
			route = &bmp.RouteConfiguration{ASPath: path, Communities: communities}
			groups[key] = route
		}
		route.Prefixes = append(route.Prefixes, prefix)
	}

	// The catch-all entries have no AS number. They give the default routes,
	// through the interface of the first rule matching any AS number.
	for _, info := range meta.RemotePrefixes {
		// A prefix can be carried in both directions, by different rules. An
		// exporter only advertises a prefix once, so the first interface found
		// wins.
		for _, direction := range []flowmodel.Direction{
			flowmodel.DirectionInbound, flowmodel.DirectionOutbound} {
			ifIndex, ok := mapping.externalInterface(exporter, info.ASN, direction)
			if !ok {
				continue
			}
			iface := config.Exporters[exporter].Interfaces[ifIndex]
			path := []uint32{}
			if iface.ASN != 0 && iface.ASN != info.ASN {
				path = append(path, iface.ASN)
			}
			if info.ASN != 0 {
				path = append(path, info.ASN)
			}
			if len(path) > 0 {
				add(info.Prefix, path, iface.Communities)
				break
			}
		}
	}
	for _, info := range meta.LocalPrefixes {
		if info.Prefix.Bits() == 0 {
			continue
		}
		add(info.Prefix, []uint32{config.LocalASN}, nil)
	}

	routes := make([]bmp.RouteConfiguration, 0, len(groups))
	for _, key := range slices.Sorted(maps.Keys(groups)) {
		route := *groups[key]
		slices.SortFunc(route.Prefixes, netip.Prefix.Compare)
		// A BGP update message is limited to 4096 bytes.
		for prefixes := range slices.Chunk(route.Prefixes, 200) {
			route.Prefixes = prefixes
			routes = append(routes, route)
		}
	}
	return routes
}
