// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package filter

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/schema"
)

func TestValidFilter(t *testing.T) {
	cases := []struct {
		Input   string
		Output  string
		MetaIn  Meta
		MetaOut Meta
	}{
		{Input: `ExporterName = 'something'`, Output: `ExporterName = 'something'`},
		{
			Input: `ExporterName = 'something'`, Output: `ExporterName = 'something'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
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
		},
		{
			Input:  `ExporterAddress << 2001:db8::c000/115`,
			Output: `ExporterAddress BETWEEN toIPv6('2001:db8::c000') AND toIPv6('2001:db8::dfff')`,
		},
		{
			Input:  `ExporterAddress << 192.168.0.0/24`,
			Output: `ExporterAddress BETWEEN toIPv6('::ffff:192.168.0.0') AND toIPv6('::ffff:192.168.0.255')`,
		},
		{
			Input:   `DstAddr << 192.168.0.0/24`,
			Output:  `DstAddr BETWEEN toIPv6('::ffff:192.168.0.0') AND toIPv6('::ffff:192.168.0.255')`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input:   `DstAddr << 192.168.0.0/24`,
			Output:  `SrcAddr BETWEEN toIPv6('::ffff:192.168.0.0') AND toIPv6('::ffff:192.168.0.255')`,
			MetaIn:  Meta{ReverseDirection: true},
			MetaOut: Meta{ReverseDirection: true, MainTableRequired: true},
		},
		{
			Input:   `SrcAddr << 192.168.0.1/24`,
			Output:  `SrcAddr BETWEEN toIPv6('::ffff:192.168.0.0') AND toIPv6('::ffff:192.168.0.255')`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input:   `DstAddr !<< 192.168.0.0/24`,
			Output:  `DstAddr NOT BETWEEN toIPv6('::ffff:192.168.0.0') AND toIPv6('::ffff:192.168.0.255')`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input:   `DstAddr !<< 192.168.0.128/27`,
			Output:  `DstAddr NOT BETWEEN toIPv6('::ffff:192.168.0.128') AND toIPv6('::ffff:192.168.0.159')`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input:   `DstNetPrefix = 192.168.0.128/27`,
			Output:  `DstAddr BETWEEN toIPv6('::ffff:192.168.0.128') AND toIPv6('::ffff:192.168.0.159') AND DstNetMask = 27`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input:   `SrcNetPrefix = 192.168.0.128/27`,
			Output:  `SrcAddr BETWEEN toIPv6('::ffff:192.168.0.128') AND toIPv6('::ffff:192.168.0.159') AND SrcNetMask = 27`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input:   `SrcNetPrefix = 2001:db8::/48`,
			Output:  `SrcAddr BETWEEN toIPv6('2001:db8::') AND toIPv6('2001:db8:0:ffff:ffff:ffff:ffff:ffff') AND SrcNetMask = 48`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{Input: `ExporterGroup= "group"`, Output: `ExporterGroup = 'group'`},
		{
			Input: `SrcAddr=203.0.113.1`, Output: `SrcAddr = toIPv6('203.0.113.1')`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input: `DstAddr=203.0.113.2`, Output: `DstAddr = toIPv6('203.0.113.2')`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input: `SrcAddr IN (203.0.113.1)`, Output: `SrcAddr IN (toIPv6('203.0.113.1'))`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input: `SrcAddr IN (203.0.113.1, 2001:db8::1)`, Output: `SrcAddr IN (toIPv6('203.0.113.1'), toIPv6('2001:db8::1'))`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{Input: `SrcNetName="alpha"`, Output: `SrcNetName = 'alpha'`},
		{Input: `DstNetName="alpha"`, Output: `DstNetName = 'alpha'`},
		{Input: `DstNetRole="stuff"`, Output: `DstNetRole = 'stuff'`},
		{Input: `SrcNetTenant="mobile"`, Output: `SrcNetTenant = 'mobile'`},
		{Input: `SrcAS=12322`, Output: `SrcAS = 12322`},
		{Input: `SrcAS=AS12322`, Output: `SrcAS = 12322`},
		{
			Input: `SrcAS=AS12322`, Output: `DstAS = 12322`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `SrcAS=as12322`, Output: `SrcAS = 12322`},
		{Input: `SrcAS IN(12322, 29447)`, Output: `SrcAS IN (12322, 29447)`},
		{Input: `SrcAS IN( 12322  , 29447  )`, Output: `SrcAS IN (12322, 29447)`},
		{Input: `SrcAS NOTIN(12322, 29447)`, Output: `SrcAS NOT IN (12322, 29447)`},
		{Input: `SrcAS NOTIN (AS12322, 29447)`, Output: `SrcAS NOT IN (12322, 29447)`},
		{Input: `DstAS=12322`, Output: `DstAS = 12322`},
		{Input: `SrcCountry='FR'`, Output: `SrcCountry = 'FR'`},
		{
			Input: `SrcCountry='FR'`, Output: `DstCountry = 'FR'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `DstCountry='FR'`, Output: `DstCountry = 'FR'`},
		{
			Input: `DstCountry='FR'`, Output: `SrcCountry = 'FR'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `InIfName='Gi0/0/0/1'`, Output: `InIfName = 'Gi0/0/0/1'`},
		{
			Input: `InIfName='Gi0/0/0/1'`, Output: `OutIfName = 'Gi0/0/0/1'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `OutIfName = 'Gi0/0/0/1'`, Output: `OutIfName = 'Gi0/0/0/1'`},
		{
			Input: `OutIfName = 'Gi0/0/0/1'`, Output: `InIfName = 'Gi0/0/0/1'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `InIfDescription='Some description'`, Output: `InIfDescription = 'Some description'`},
		{
			Input: `InIfDescription='Some description'`, Output: `OutIfDescription = 'Some description'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `OutIfDescription='Some other description'`, Output: `OutIfDescription = 'Some other description'`},
		{
			Input: `OutIfDescription='Some other description'`, Output: `InIfDescription = 'Some other description'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `InIfSpeed>=1000`, Output: `InIfSpeed >= 1000`},
		{
			Input: `InIfSpeed>=1000`, Output: `OutIfSpeed >= 1000`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `InIfSpeed!=1000`, Output: `InIfSpeed != 1000`},
		{Input: `InIfSpeed<1000`, Output: `InIfSpeed < 1000`},
		{Input: `OutIfSpeed!=1000`, Output: `OutIfSpeed != 1000`},
		{
			Input: `OutIfSpeed!=1000`, Output: `InIfSpeed != 1000`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `InIfConnectivity = 'pni'`, Output: `InIfConnectivity = 'pni'`},
		{
			Input: `InIfConnectivity = 'pni'`, Output: `OutIfConnectivity = 'pni'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `OutIfConnectivity = 'ix'`, Output: `OutIfConnectivity = 'ix'`},
		{
			Input: `OutIfConnectivity = 'ix'`, Output: `InIfConnectivity = 'ix'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `InIfProvider = 'cogent'`, Output: `InIfProvider = 'cogent'`},
		{
			Input: `InIfProvider = 'cogent'`, Output: `OutIfProvider = 'cogent'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `OutIfProvider = 'telia'`, Output: `OutIfProvider = 'telia'`},
		{
			Input: `OutIfProvider = 'telia'`, Output: `InIfProvider = 'telia'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `InIfBoundary = external`, Output: `InIfBoundary = 'external'`},
		{
			Input: `InIfBoundary = external`, Output: `OutIfBoundary = 'external'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `InIfBoundary = EXTERNAL`, Output: `InIfBoundary = 'external'`},
		{
			Input: `InIfBoundary = EXTERNAL`, Output: `OutIfBoundary = 'external'`,
			MetaIn: Meta{ReverseDirection: true}, MetaOut: Meta{ReverseDirection: true},
		},
		{Input: `OutIfBoundary != internal`, Output: `OutIfBoundary != 'internal'`},
		{Input: `EType = ipv4`, Output: `EType = 2048`},
		{Input: `EType != ipv6`, Output: `EType != 34525`},
		{Input: `Proto = 1`, Output: `Proto = 1`},
		{Input: `Proto = 'gre'`, Output: `dictGetOrDefault('protocols', 'name', Proto, '???') = 'gre'`},
		{
			Input: `SrcPort = 80`, Output: `SrcPort = 80`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input: `SrcPort = 80`, Output: `DstPort = 80`,
			MetaIn:  Meta{ReverseDirection: true},
			MetaOut: Meta{ReverseDirection: true, MainTableRequired: true},
		},
		{
			Input: `DstPort > 1024`, Output: `DstPort > 1024`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{Input: `ForwardingStatus >= 128`, Output: `ForwardingStatus >= 128`},
		{Input: `PacketSize > 1500`, Output: `PacketSize > 1500`},
		{
			Input: `DstPort > 1024 AND SrcPort < 1024`, Output: `DstPort > 1024 AND SrcPort < 1024`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input: `DstPort > 1024 OR SrcPort < 1024`, Output: `DstPort > 1024 OR SrcPort < 1024`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input: `NOT DstPort > 1024 AND SrcPort < 1024`, Output: `NOT DstPort > 1024 AND SrcPort < 1024`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input: `not DstPort > 1024 and SrcPort < 1024`, Output: `NOT DstPort > 1024 AND SrcPort < 1024`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input:   `DstPort > 1024 AND SrcPort < 1024 OR InIfSpeed >= 1000`,
			Output:  `DstPort > 1024 AND SrcPort < 1024 OR InIfSpeed >= 1000`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input:   `DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`,
			Output:  `DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input:   `  DstPort >   1024   AND   (  SrcPort   <   1024   OR   InIfSpeed   >=   1000   )  `,
			Output:  `DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input:   `DstPort > 1024 AND(SrcPort < 1024 OR InIfSpeed >= 1000)`,
			Output:  `DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
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
		},
		{
			Input:  `InIfDescription = "This contains a -- comment" -- nope`,
			Output: `InIfDescription = 'This contains a -- comment'`,
		},
		{
			Input:  `InIfDescription = "This contains a /* comment"`,
			Output: `InIfDescription = 'This contains a /* comment'`,
		},
		{Input: `OutIfProvider /* That's the output provider */ = 'telia'`, Output: `OutIfProvider = 'telia'`},
		{
			Input: `OutIfProvider /* That's the
output provider */ = 'telia'`,
			Output: `OutIfProvider = 'telia'`,
		},
		{Input: `DstASPath = 65000`, Output: `has(DstASPath, 65000)`, MetaOut: Meta{MainTableRequired: true}},
		{Input: `DstASPath != 65000`, Output: `NOT has(DstASPath, 65000)`, MetaOut: Meta{MainTableRequired: true}},
		{Input: `DstCommunities = 65000:100`, Output: `has(DstCommunities, 4259840100)`, MetaOut: Meta{MainTableRequired: true}},
		{Input: `DstCommunities != 65000:100`, Output: `NOT has(DstCommunities, 4259840100)`, MetaOut: Meta{MainTableRequired: true}},
		{Input: `DstCommunities = 65000:100:200`, Output: `has(DstLargeCommunities, bitShiftLeft(65000::UInt128, 64) + bitShiftLeft(100::UInt128, 32) + 200::UInt128)`, MetaOut: Meta{MainTableRequired: true}},
		{Input: `DstCommunities != 65000:100:200`, Output: `NOT has(DstLargeCommunities, bitShiftLeft(65000::UInt128, 64) + bitShiftLeft(100::UInt128, 32) + 200::UInt128)`, MetaOut: Meta{MainTableRequired: true}},
		{Input: `SrcVlan = 1000`, Output: `SrcVlan = 1000`},
		{Input: `DstVlan = 1000`, Output: `DstVlan = 1000`},
		{
			Input: `SrcAddrNAT = 203.0.113.4`, Output: `SrcAddrNAT = toIPv6('203.0.113.4')`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input: `DstAddrNAT = 203.0.113.4`, Output: `DstAddrNAT = toIPv6('203.0.113.4')`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input: `SrcPortNAT = 22`, Output: `SrcPortNAT = 22`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{
			Input: `DstPortNAT = 22`, Output: `DstPortNAT = 22`,
			MetaOut: Meta{MainTableRequired: true},
		},
		{Input: `SrcMAC = 00:11:22:33:44:55`, Output: `SrcMAC = MACStringToNum('00:11:22:33:44:55')`},
		{Input: `DstMAC = 00:11:22:33:44:55`, Output: `DstMAC = MACStringToNum('00:11:22:33:44:55')`},
		{Input: `SrcMAC != 00:0c:fF:33:44:55`, Output: `SrcMAC != MACStringToNum('00:0c:ff:33:44:55')`},
		{Input: `SrcMAC = 0000.5e00.5301`, Output: `SrcMAC = MACStringToNum('00:00:5e:00:53:01')`},
		{Input: `ipttl > 50`, Output: `IPTTL > 50`},
		{Input: `iptos = 0`, Output: `IPTos = 0`},
		{Input: `ipfragmentid != 0`, Output: `IPFragmentID != 0`},
		{Input: `ipfragmentoffset = 3`, Output: `IPFragmentOffset = 3`},
		{Input: `ipv6flowlabel = 0`, Output: `IPv6FlowLabel = 0`},
		{Input: `tcpflags = 2`, Output: `TCPFlags = 2`},
		{Input: `icmpv4type = 8 AND icmpv4code = 0`, Output: `ICMPv4Type = 8 AND ICMPv4Code = 0`},
		{Input: `icmpv6type = 8 or icmpv6code = 0`, Output: `ICMPv6Type = 8 OR ICMPv6Code = 0`},
		{Input: `icmpv6 = "echo-reply"`, Output: `ICMPv6 = 'echo-reply'`},
		{Input: `SrcAddrDimensionAttribute = "Test"`, Output: `SrcAddrDimensionAttribute = 'Test'`},
		{Input: `DstAddrDimensionAttribute = "Test"`, Output: `DstAddrDimensionAttribute = 'Test'`},
		{Input: `DstAddrRole = "Test"`, Output: `DstAddrRole = 'Test'`},
		{Input: `DstAddrPriority = 200`, Output: `DstAddrPriority = 200`},
		{Input: `DstAddrSibling = 2001:db8::1`, Output: `DstAddrSibling = toIPv6('2001:db8::1')`},
		{Input: `SrcAddrDimensionAttribute IN ("Test", "None")`, Output: `SrcAddrDimensionAttribute IN ('Test', 'None')`},
	}
	config := schema.DefaultConfiguration()
	config.CustomDictionaries = make(map[string]schema.CustomDict)
	config.CustomDictionaries["test"] = schema.CustomDict{
		Keys: []schema.CustomDictKey{
			{Name: "SrcAddr", Type: "String"},
		},
		Attributes: []schema.CustomDictAttribute{
			{Name: "csv_col_name", Type: "String", Label: "DimensionAttribute"},
			{Name: "role", Type: "String"},
			{Name: "priority", Type: "UInt16"},
			{Name: "sibling", Type: "IPv6"},
		},
		Source:     "test.csv",
		Dimensions: []string{"SrcAddr", "DstAddr"},
	}
	s, _ := schema.New(config)
	s = s.EnableAllColumns()
	for _, tc := range cases {
		tc.MetaIn.Schema = s
		tc.MetaOut.Schema = tc.MetaIn.Schema
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

func TestValidMaterializedFilter(t *testing.T) {
	cases := []struct {
		Input   string
		Output  string
		MetaIn  Meta
		MetaOut Meta
	}{
		{
			Input:   `DstNetPrefix = 192.168.0.128/27`,
			Output:  `DstNetPrefix = '192.168.0.128/27'`,
			MetaOut: Meta{MainTableRequired: false},
		},
		{
			Input:   `SrcNetPrefix = 192.168.0.128/27`,
			Output:  `SrcNetPrefix = '192.168.0.128/27'`,
			MetaOut: Meta{MainTableRequired: false},
		},
		{
			Input:   `SrcNetPrefix = 2001:db8::/48`,
			Output:  `SrcNetPrefix = '2001:db8::/48'`,
			MetaOut: Meta{MainTableRequired: false},
		},
	}
	for _, tc := range cases {
		s := schema.NewMock(t).EnableAllColumns()
		cd, _ := s.Schema.LookupColumnByKey(schema.ColumnDstNetPrefix)
		cd.ClickHouseMaterialized = true
		cs, _ := s.Schema.LookupColumnByKey(schema.ColumnSrcNetPrefix)
		cs.ClickHouseMaterialized = true

		tc.MetaIn.Schema = s
		tc.MetaOut.Schema = tc.MetaIn.Schema
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
		Input     string
		EnableAll bool
	}{
		{Input: `ExporterName`},
		{Input: `ExporterName = `},
		{Input: `ExporterName = 'something`},
		{Input: `ExporterName='something"`},
		{Input: `ExporterNamee="something"`},
		{Input: `ExporterName>"something"`},
		{Input: `ExporterAddress=203.0.113`},
		{Input: `ExporterAddress=2001:db8`},
		{Input: `ExporterAddress="2001:db8:0::1"`},
		{Input: `SrcAS=12322a`},
		{Input: `SrcAS=785473854857857485784`},
		{Input: `EType = ipv7`},
		{Input: `Proto = 100 AND`},
		{Input: `AND Proto = 100`},
		{Input: `Proto = 100AND Proto = 100`},
		{Input: `Proto = 100 ANDProto = 100`},
		{Input: `Proto = 100 AND (Proto = 100`},
		{Input: `Proto = 100 /* Hello !`},
		{Input: `SrcAS IN (AS12322, 29447`},
		{Input: `SrcAS IN (AS12322 29447)`},
		{Input: `SrcAS IN (AS12322,`},
		{Input: `SrcVlan = 1000`},
		{Input: `DstVlan = 1000`},
		{Input: `SrcMAC = 00:11:22:33:44:55:66`, EnableAll: true},
		{Input: `SrcAddrDimensionAttribute = 8`},
		{Input: `InvalidDimensionAttribute = "Test"`},
	}
	config := schema.DefaultConfiguration()
	config.CustomDictionaries = make(map[string]schema.CustomDict)
	config.CustomDictionaries["test"] = schema.CustomDict{
		Keys: []schema.CustomDictKey{
			{Name: "SrcAddr", Type: "string"},
		},
		Attributes: []schema.CustomDictAttribute{
			{Name: "csv_col_name", Type: "string", Label: "DimensionAttribute"},
			{Name: "role", Type: "string"},
		},
		Source:     "test.csv",
		Dimensions: []string{"SrcAddr", "DstAddr"},
	}

	s, _ := schema.New(config)

	for _, tc := range cases {
		sch := schema.NewMock(t)
		if tc.EnableAll {
			s = s.EnableAllColumns()
		}
		out, err := Parse("", []byte(tc.Input), GlobalStore("meta", &Meta{Schema: sch}))
		if err == nil {
			t.Errorf("Parse(%q) didn't throw an error (got %s)", tc.Input, out)
		}
	}
}
