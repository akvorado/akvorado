// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"testing"

	"akvorado/common/helpers"

	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
)

func TestASPathFlat(t *testing.T) {
	cases := []struct {
		AsPath   *bgp.PathAttributeAsPath
		Expected []uint32
	}{
		{
			AsPath:   bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{}),
			Expected: []uint32{},
		}, {
			AsPath: bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{
				bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint16{65402, 65403, 65404}),
			}),
			Expected: []uint32{65402, 65403, 65404},
		}, {
			AsPath: bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{
				bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint32{65402, 65536, 65537}),
			}),
			Expected: []uint32{65402, 65536, 65537},
		}, {
			AsPath: bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{
				bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SET, []uint16{65402, 65403, 65404}),
			}),
			Expected: []uint32{65402},
		}, {
			AsPath: bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{
				bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_CONFED_SEQ, []uint16{65402, 65403, 65404}),
			}),
			Expected: []uint32{65402, 65403, 65404},
		}, {
			AsPath: bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{
				bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_CONFED_SET, []uint16{65402, 65403, 65404}),
			}),
			Expected: []uint32{65402},
		}, {
			AsPath: bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{
				bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint16{65402, 65403, 65404}),
				bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SET, []uint16{65405, 65406}),
				bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_CONFED_SEQ, []uint16{65407, 65408}),
				bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_CONFED_SET, []uint16{65409, 65410}),
				bgp.NewAsPathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint16{65411}),
			}),
			Expected: []uint32{65402, 65403, 65404, 65405, 65407, 65408, 65409, 65411},
		},
	}
	for _, tc := range cases {
		t.Run(tc.AsPath.String(), func(t *testing.T) {
			got := asPathFlat(tc.AsPath)
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("asPathFlat() (-got, +want):\n%s", diff)
			}
		})
	}
}
