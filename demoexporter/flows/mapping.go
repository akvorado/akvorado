// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"errors"
	"fmt"
	"slices"

	"akvorado/common/schema"
	"akvorado/demoexporter/flowmodel"
)

// compiledMapping is the mapping configuration prepared for fast and
// deterministic assignment of flows to exporters and interfaces.
type compiledMapping struct {
	rules []compiledRule
	// interfacesByBoundary lists the sorted ifIndexes of each exporter for each
	// boundary.
	interfacesByBoundary map[string]map[schema.InterfaceBoundary][]int
}

type compiledRule struct {
	asns      map[uint32]bool
	direction MatchDirection
	assigns   []AssignConfiguration
	// cumulative holds the cumulative normalized weights of the assign entries.
	cumulative []float64
}

// newMapping compiles and checks the mapping rules.
func newMapping(config *Configuration) (*compiledMapping, error) {
	m := compiledMapping{
		interfacesByBoundary: map[string]map[schema.InterfaceBoundary][]int{},
	}
	for name, exporter := range config.Exporters {
		byBoundary := map[schema.InterfaceBoundary][]int{}
		for ifIndex, iface := range exporter.Interfaces {
			byBoundary[iface.Boundary] = append(byBoundary[iface.Boundary], ifIndex)
		}
		for _, ifIndexes := range byBoundary {
			slices.Sort(ifIndexes)
		}
		if len(byBoundary[schema.InterfaceBoundaryInternal]) == 0 {
			return nil, fmt.Errorf("exporter %q has no internal interface", name)
		}
		m.interfacesByBoundary[name] = byBoundary
	}
	assigned := false
	for i, rule := range config.Mapping {
		compiled := compiledRule{
			asns:      map[uint32]bool{},
			direction: rule.Direction,
			assigns:   rule.Assign,
		}
		for _, asn := range rule.ASN {
			compiled.asns[asn] = true
		}
		total := 0.0
		for _, assign := range rule.Assign {
			exporter, ok := config.Exporters[assign.Exporter]
			if !ok {
				return nil, fmt.Errorf("mapping rule %d assigns to unknown exporter %q",
					i+1, assign.Exporter)
			}
			iface, ok := exporter.Interfaces[assign.Interface]
			if !ok {
				return nil, fmt.Errorf("mapping rule %d assigns to unknown interface %d of exporter %q",
					i+1, assign.Interface, assign.Exporter)
			}
			if iface.Boundary != schema.InterfaceBoundaryExternal {
				return nil, fmt.Errorf("mapping rule %d assigns to interface %d of exporter %q, which is not external",
					i+1, assign.Interface, assign.Exporter)
			}
			assigned = assigned || assign.Exporter == config.Exporter
			total += assign.Weight
			compiled.cumulative = append(compiled.cumulative, total)
		}
		for j := range compiled.cumulative {
			compiled.cumulative[j] /= total
		}
		m.rules = append(m.rules, compiled)
	}
	if !assigned {
		return nil, fmt.Errorf("no mapping rule assigns flows to exporter %q", config.Exporter)
	}
	for _, direction := range []flowmodel.Direction{flowmodel.DirectionInbound, flowmodel.DirectionOutbound} {
		// AS 0 only matches a rule for any AS number.
		if _, _, ok := m.assign(&flowmodel.Flow{Direction: direction}); !ok {
			return nil, errors.New("the mapping needs a rule matching any AS number for both directions")
		}
	}
	return &m, nil
}

// matches tells if a rule handles the traffic of one remote AS number in one
// direction.
func (r *compiledRule) matches(asn uint32, direction flowmodel.Direction) bool {
	if r.direction == MatchDirectionInbound && direction != flowmodel.DirectionInbound {
		return false
	}
	if r.direction == MatchDirectionOutbound && direction != flowmodel.DirectionOutbound {
		return false
	}
	if len(r.asns) > 0 && !r.asns[asn] {
		return false
	}
	return true
}

// assign returns the exporter and the external interface carrying a flow. The
// result only depends on the flow, so it is the same on all the instances.
func (m *compiledMapping) assign(flow *flowmodel.Flow) (string, int, bool) {
	for i := range m.rules {
		rule := &m.rules[i]
		if !rule.matches(flow.RemoteASN, flow.Direction) {
			continue
		}
		// Pick an assign entry from the flow key. The key is uniform,
		// so the weights are respected.
		u := float64(flow.Key>>11) / (1 << 53)
		for j, bound := range rule.cumulative {
			if u < bound {
				return rule.assigns[j].Exporter, rule.assigns[j].Interface, true
			}
		}
		last := len(rule.assigns) - 1
		return rule.assigns[last].Exporter, rule.assigns[last].Interface, true
	}
	return "", 0, false
}

// externalInterface returns the external interface of an exporter carrying most
// of the traffic of one remote AS number in one direction. It follows the same
// rules as assign(): the first matching rule wins, even when it sends the
// traffic to another exporter.
func (m *compiledMapping) externalInterface(exporter string, asn uint32, direction flowmodel.Direction) (int, bool) {
	for i := range m.rules {
		rule := &m.rules[i]
		if !rule.matches(asn, direction) {
			continue
		}
		best := AssignConfiguration{}
		for _, assign := range rule.assigns {
			if assign.Exporter == exporter && assign.Weight > best.Weight {
				best = assign
			}
		}
		return best.Interface, best.Weight > 0
	}
	return 0, false
}

// otherInterface returns the interface for the non-Internet side of a flow,
// using the boundary the model sampled for that side. It is never the external
// interface carrying the flow.
func (m *compiledMapping) otherInterface(exporter string, boundary schema.InterfaceBoundary, externalIf int, key uint64) int {
	// Derive a second uniform value from the key.
	key = (key ^ key>>33) * 0x9e3779b97f4a7c15
	byBoundary := m.interfacesByBoundary[exporter]
	if candidates := byBoundary[boundary]; len(candidates) > 0 {
		idx := int(key % uint64(len(candidates)))
		if candidates[idx] == externalIf {
			idx = (idx + 1) % len(candidates)
		}
		if candidates[idx] != externalIf {
			return candidates[idx]
		}
	}
	candidates := byBoundary[schema.InterfaceBoundaryInternal]
	return candidates[key%uint64(len(candidates))]
}
