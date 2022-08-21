// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import "github.com/osrg/gobgp/v3/pkg/packet/bgp"

// asPathFlat transforms an AS path to a flat AS path: first value of
// a set is used, confed seq is considered as a regular seq.
func asPathFlat(aspath *bgp.PathAttributeAsPath) []uint32 {
	s := []uint32{}
	for _, param := range aspath.Value {
		segType := param.GetType()
		asList := param.GetAS()

		switch segType {
		case bgp.BGP_ASPATH_ATTR_TYPE_CONFED_SET, bgp.BGP_ASPATH_ATTR_TYPE_SET:
			asList = asList[:1]
		}
		for _, as := range asList {
			s = append(s, as)
		}
	}
	return s
}
