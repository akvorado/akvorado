// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package filter

import (
	"testing"

	"akvorado/common/helpers"
)

func TestValidFilter(t *testing.T) {
	cases := []struct {
		Input   string
		Output  string
		MetaIn  Meta
		MetaOut Meta
	}{
		{Input: `ExporterName = 'something'`, Output: `ExporterName = 'something'`},
		{Input: `ExporterName = 'something'`, Output: `ExporterName = 'something'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `exportername = 'something'`, Output: `ExporterName = 'something'`},
		{Input: `ExporterName='something'`, Output: `ExporterName = 'something'`},
		{Input: `ExporterName="something"`, Output: `ExporterName = 'something'`},
		{Input: `ExporterName="something'"`, Output: `ExporterName = 'something\''`},
		{Input: `ExporterName="something\"`, Output: `ExporterName = 'something\\'`},
		{Input: `ExporterName!="something"`, Output: `ExporterName != 'something'`},
		{Input: `ExporterName IN ("something")`, Output: `ExporterName IN ('something')`},
		{Input: `ExporterName IN ("something","something else")`, Output: `ExporterName IN ('something', 'something else')`},
		{Input: `ExporterName LIKE "something%"`, Output: `ExporterName LIKE 'something%'`},
		{Input: `ExporterName UNLIKE "something%"`, Output: `ExporterName NOT LIKE 'something%'`},
		{Input: `ExporterName IUNLIKE "something%"`, Output: `ExporterName NOT ILIKE 'something%'`},
		{Input: `ExporterName="something with spaces"`, Output: `ExporterName = 'something with spaces'`},
		{Input: `ExporterName="something with 'quotes'"`, Output: `ExporterName = 'something with \'quotes\''`},
		{Input: `ExporterAddress=203.0.113.1`, Output: `ExporterAddress = toIPv6('203.0.113.1')`},
		{Input: `ExporterAddress=2001:db8::1`, Output: `ExporterAddress = toIPv6('2001:db8::1')`},
		{Input: `ExporterAddress=2001:db8:0::1`, Output: `ExporterAddress = toIPv6('2001:db8::1')`},
		{
			Input:  `ExporterAddress << 2001:db8:0::/64`,
			Output: `ExporterAddress BETWEEN toIPv6('2001:db8::') AND toIPv6('2001:db8::ffff:ffff:ffff:ffff')`,
		}, {
			Input:  `ExporterAddress << 2001:db8::c000/115`,
			Output: `ExporterAddress BETWEEN toIPv6('2001:db8::c000') AND toIPv6('2001:db8::dfff')`,
		}, {
			Input:  `ExporterAddress << 192.168.0.0/24`,
			Output: `ExporterAddress BETWEEN toIPv6('::ffff:192.168.0.0') AND toIPv6('::ffff:192.168.0.255')`,
		}, {
			Input:   `DstAddr << 192.168.0.0/24`,
			Output:  `DstAddr BETWEEN toIPv6('::ffff:192.168.0.0') AND toIPv6('::ffff:192.168.0.255')`,
			MetaOut: Meta{MainTableRequired: true},
		}, {
			Input:   `DstAddr << 192.168.0.0/24`,
			Output:  `SrcAddr BETWEEN toIPv6('::ffff:192.168.0.0') AND toIPv6('::ffff:192.168.0.255')`,
			MetaIn:  Meta{ReverseDirection: true},
			MetaOut: Meta{ReverseDirection: true, MainTableRequired: true},
		}, {
			Input:   `SrcAddr << 192.168.0.1/24`,
			Output:  `SrcAddr BETWEEN toIPv6('::ffff:192.168.0.0') AND toIPv6('::ffff:192.168.0.255')`,
			MetaOut: Meta{MainTableRequired: true},
		}, {
			Input:   `DstAddr !<< 192.168.0.0/24`,
			Output:  `DstAddr NOT BETWEEN toIPv6('::ffff:192.168.0.0') AND toIPv6('::ffff:192.168.0.255')`,
			MetaOut: Meta{MainTableRequired: true},
		}, {
			Input:   `DstAddr !<< 192.168.0.128/27`,
			Output:  `DstAddr NOT BETWEEN toIPv6('::ffff:192.168.0.128') AND toIPv6('::ffff:192.168.0.159')`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{Input: `ExporterGroup= "group"`, Output: `ExporterGroup = 'group'`},
		{Input: `SrcAddr=203.0.113.1`, Output: `SrcAddr = toIPv6('203.0.113.1')`,
			MetaOut: Meta{MainTableRequired: true}},
		{Input: `DstAddr=203.0.113.2`, Output: `DstAddr = toIPv6('203.0.113.2')`,
			MetaOut: Meta{MainTableRequired: true}},
		{Input: `SrcNetName="alpha"`, Output: `SrcNetName = 'alpha'`},
		{Input: `DstNetName="alpha"`, Output: `DstNetName = 'alpha'`},
		{Input: `DstNetRole="stuff"`, Output: `DstNetRole = 'stuff'`},
		{Input: `SrcNetTenant="mobile"`, Output: `SrcNetTenant = 'mobile'`},
		{Input: `SrcAS=12322`, Output: `SrcAS = 12322`},
		{Input: `SrcAS=AS12322`, Output: `SrcAS = 12322`},
		{Input: `SrcAS=AS12322`, Output: `DstAS = 12322`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `SrcAS=as12322`, Output: `SrcAS = 12322`},
		{Input: `SrcAS IN(12322, 29447)`, Output: `SrcAS IN (12322, 29447)`},
		{Input: `SrcAS IN( 12322  , 29447  )`, Output: `SrcAS IN (12322, 29447)`},
		{Input: `SrcAS NOTIN(12322, 29447)`, Output: `SrcAS NOT IN (12322, 29447)`},
		{Input: `SrcAS NOTIN (AS12322, 29447)`, Output: `SrcAS NOT IN (12322, 29447)`},
		{Input: `DstAS=12322`, Output: `DstAS = 12322`},
		{Input: `SrcCountry='FR'`, Output: `SrcCountry = 'FR'`},
		{Input: `SrcCountry='FR'`, Output: `DstCountry = 'FR'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `DstCountry='FR'`, Output: `DstCountry = 'FR'`},
		{Input: `DstCountry='FR'`, Output: `SrcCountry = 'FR'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `InIfName='Gi0/0/0/1'`, Output: `InIfName = 'Gi0/0/0/1'`},
		{Input: `InIfName='Gi0/0/0/1'`, Output: `OutIfName = 'Gi0/0/0/1'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `OutIfName = 'Gi0/0/0/1'`, Output: `OutIfName = 'Gi0/0/0/1'`},
		{Input: `OutIfName = 'Gi0/0/0/1'`, Output: `InIfName = 'Gi0/0/0/1'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `InIfDescription='Some description'`, Output: `InIfDescription = 'Some description'`},
		{Input: `InIfDescription='Some description'`, Output: `OutIfDescription = 'Some description'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `OutIfDescription='Some other description'`, Output: `OutIfDescription = 'Some other description'`},
		{Input: `OutIfDescription='Some other description'`, Output: `InIfDescription = 'Some other description'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `InIfSpeed>=1000`, Output: `InIfSpeed >= 1000`},
		{Input: `InIfSpeed>=1000`, Output: `OutIfSpeed >= 1000`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `InIfSpeed!=1000`, Output: `InIfSpeed != 1000`},
		{Input: `InIfSpeed<1000`, Output: `InIfSpeed < 1000`},
		{Input: `OutIfSpeed!=1000`, Output: `OutIfSpeed != 1000`},
		{Input: `OutIfSpeed!=1000`, Output: `InIfSpeed != 1000`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `InIfConnectivity = 'pni'`, Output: `InIfConnectivity = 'pni'`},
		{Input: `InIfConnectivity = 'pni'`, Output: `OutIfConnectivity = 'pni'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `OutIfConnectivity = 'ix'`, Output: `OutIfConnectivity = 'ix'`},
		{Input: `OutIfConnectivity = 'ix'`, Output: `InIfConnectivity = 'ix'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `InIfProvider = 'cogent'`, Output: `InIfProvider = 'cogent'`},
		{Input: `InIfProvider = 'cogent'`, Output: `OutIfProvider = 'cogent'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `OutIfProvider = 'telia'`, Output: `OutIfProvider = 'telia'`},
		{Input: `OutIfProvider = 'telia'`, Output: `InIfProvider = 'telia'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `InIfBoundary = external`, Output: `InIfBoundary = 'external'`},
		{Input: `InIfBoundary = external`, Output: `OutIfBoundary = 'external'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `InIfBoundary = EXTERNAL`, Output: `InIfBoundary = 'external'`},
		{Input: `InIfBoundary = EXTERNAL`, Output: `OutIfBoundary = 'external'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true}},
		{Input: `OutIfBoundary != internal`, Output: `OutIfBoundary != 'internal'`},
		{Input: `EType = ipv4`, Output: `EType = 2048`},
		{Input: `EType != ipv6`, Output: `EType != 34525`},
		{Input: `Proto = 1`, Output: `Proto = 1`},
		{Input: `Proto = 'gre'`, Output: `dictGetOrDefault('protocols', 'name', Proto, '???') = 'gre'`},
		{Input: `SrcPort = 80`, Output: `SrcPort = 80`,
			MetaOut: Meta{MainTableRequired: true}},
		{Input: `SrcPort = 80`, Output: `DstPort = 80`,
			MetaIn:  Meta{ReverseDirection: true},
			MetaOut: Meta{ReverseDirection: true, MainTableRequired: true}},
		{Input: `DstPort > 1024`, Output: `DstPort > 1024`,
			MetaOut: Meta{MainTableRequired: true}},
		{Input: `ForwardingStatus >= 128`, Output: `ForwardingStatus >= 128`},
		{Input: `PacketSize > 1500`, Output: `Bytes/Packets > 1500`},
		{Input: `DstPort > 1024 AND SrcPort < 1024`, Output: `DstPort > 1024 AND SrcPort < 1024`,
			MetaOut: Meta{MainTableRequired: true}},
		{Input: `DstPort > 1024 OR SrcPort < 1024`, Output: `DstPort > 1024 OR SrcPort < 1024`,
			MetaOut: Meta{MainTableRequired: true}},
		{Input: `NOT DstPort > 1024 AND SrcPort < 1024`, Output: `NOT DstPort > 1024 AND SrcPort < 1024`,
			MetaOut: Meta{MainTableRequired: true}},
		{Input: `not DstPort > 1024 and SrcPort < 1024`, Output: `NOT DstPort > 1024 AND SrcPort < 1024`,
			MetaOut: Meta{MainTableRequired: true}},
		{
			Input:   `DstPort > 1024 AND SrcPort < 1024 OR InIfSpeed >= 1000`,
			Output:  `DstPort > 1024 AND SrcPort < 1024 OR InIfSpeed >= 1000`,
			MetaOut: Meta{MainTableRequired: true},
		}, {
			Input:   `DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`,
			Output:  `DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`,
			MetaOut: Meta{MainTableRequired: true},
		}, {
			Input:   `  DstPort >   1024   AND   (  SrcPort   <   1024   OR   InIfSpeed   >=   1000   )  `,
			Output:  `DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`,
			MetaOut: Meta{MainTableRequired: true},
		}, {
			Input:   `DstPort > 1024 AND(SrcPort < 1024 OR InIfSpeed >= 1000)`,
			Output:  `DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`,
			MetaOut: Meta{MainTableRequired: true},
		}, {
			Input: `DstPort > 1024
                  AND (SrcPort < 1024 OR InIfSpeed >= 1000)`,
			Output:  `DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{Input: `(ExporterAddress=203.0.113.1)`, Output: `(ExporterAddress = toIPv6('203.0.113.1'))`},
		{Input: `ForwardingStatus >= 128 -- Nothing`, Output: `ForwardingStatus >= 128`},
		{
			Input: `
-- Example of commented request
-- Here we go
DstPort > 1024 -- Non-privileged port
AND SrcAS = AS12322 -- Proxad ASN`,
			Output:  `DstPort > 1024 AND SrcAS = 12322`,
			MetaOut: Meta{MainTableRequired: true},
		}, {
			Input:  `InIfDescription = "This contains a -- comment" -- nope`,
			Output: `InIfDescription = 'This contains a -- comment'`,
		}, {
			Input:  `InIfDescription = "This contains a /* comment"`,
			Output: `InIfDescription = 'This contains a /* comment'`,
		},
		{Input: `OutIfProvider /* That's the output provider */ = 'telia'`, Output: `OutIfProvider = 'telia'`},
		{
			Input: `OutIfProvider /* That's the
output provider */ = 'telia'`,
			Output: `OutIfProvider = 'telia'`,
		},
		{Input: `DstASPath HAS 65000`, Output: `has(DstASPath, 65000)`, MetaOut: Meta{MainTableRequired: true}},
		{Input: `DstASPath HASNOT 65000`, Output: `NOT has(DstASPath, 65000)`, MetaOut: Meta{MainTableRequired: true}},
		{Input: `DstCommunities HAS 65000:100`, Output: `has(DstCommunities, 4259840100)`, MetaOut: Meta{MainTableRequired: true}},
		{Input: `DstCommunities HASNOT 65000:100`, Output: `NOT has(DstCommunities, 4259840100)`, MetaOut: Meta{MainTableRequired: true}},
		{Input: `DstCommunities HAS 65000:100:200`, Output: `has(DstLargeCommunities, bitShiftLeft(65000::UInt128, 64) + bitShiftLeft(100::UInt128, 32) + 200::UInt128)`, MetaOut: Meta{MainTableRequired: true}},
		{Input: `DstCommunities HASNOT 65000:100:200`, Output: `NOT has(DstLargeCommunities, bitShiftLeft(65000::UInt128, 64) + bitShiftLeft(100::UInt128, 32) + 200::UInt128)`, MetaOut: Meta{MainTableRequired: true}},
	}
	for _, tc := range cases {
		got, err := Parse("", []byte(tc.Input), GlobalStore("meta", &tc.MetaIn))
		if err != nil {
			t.Errorf("Parse(%q) error:\n%+v", tc.Input, err)
			continue
		}
		if diff := helpers.Diff(got.(string), tc.Output); diff != "" {
			t.Errorf("Parse(%q) (-got, +want):\n%s", tc.Input, diff)
		}
		if diff := helpers.Diff(tc.MetaIn, tc.MetaOut); diff != "" {
			t.Errorf("Parse(%q) meta (-got, +want):\n%s", tc.Input, diff)
		}
	}
}

func TestInvalidFilter(t *testing.T) {
	cases := []struct {
		Input string
	}{
		{`ExporterName`},
		{`ExporterName = `},
		{`ExporterName = 'something`},
		{`ExporterName='something"`},
		{`ExporterNamee="something"`},
		{`ExporterName>"something"`},
		{`ExporterAddress=203.0.113`},
		{`ExporterAddress=2001:db8`},
		{`ExporterAddress="2001:db8:0::1"`},
		{`SrcAS=12322a`},
		{`SrcAS=785473854857857485784`},
		{`EType = ipv7`},
		{`Proto = 1000`},
		{`SrcPort = 1000000`},
		{`ForwardingStatus >= 900`},
		{`Proto = 100 AND`},
		{`AND Proto = 100`},
		{`Proto = 100AND Proto = 100`},
		{`Proto = 100 ANDProto = 100`},
		{`Proto = 100 AND (Proto = 100`},
		{`Proto = 100 /* Hello !`},
		{`SrcAS IN (AS12322, 29447`},
		{`SrcAS IN (AS12322 29447)`},
		{`SrcAS IN (AS12322,`},
	}
	for _, tc := range cases {
		out, err := Parse("", []byte(tc.Input), GlobalStore("meta", &Meta{}))
		t.Logf("out: %v", out)
		if err == nil {
			t.Errorf("Parse(%q) didn't throw an error", tc.Input)
		}
	}
}
