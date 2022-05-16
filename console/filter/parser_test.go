package filter

import (
	"testing"

	"akvorado/common/helpers"
)

func TestValidFilter(t *testing.T) {
	cases := []struct {
		Input  string
		Output string
	}{
		{`ExporterName = 'something'`, `ExporterName = 'something'`},
		{`ExporterName='something'`, `ExporterName = 'something'`},
		{`ExporterName="something"`, `ExporterName = 'something'`},
		{`ExporterName!="something"`, `ExporterName != 'something'`},
		{`ExporterName="something with spaces"`, `ExporterName = 'something with spaces'`},
		{`ExporterName="something with 'quotes'"`, `ExporterName = 'something with \'quotes\''`},
		{`ExporterAddress=203.0.113.1`, `ExporterAddress = IPv6StringToNum('203.0.113.1')`},
		{`ExporterAddress=2001:db8::1`, `ExporterAddress = IPv6StringToNum('2001:db8::1')`},
		{`ExporterAddress=2001:db8:0::1`, `ExporterAddress = IPv6StringToNum('2001:db8::1')`},
		{`ExporterGroup= "group"`, `ExporterGroup = 'group'`},
		{`SrcAddr=203.0.113.1`, `SrcAddr = IPv6StringToNum('203.0.113.1')`},
		{`DstAddr=203.0.113.2`, `DstAddr = IPv6StringToNum('203.0.113.2')`},
		{`SrcAS=12322`, `SrcAS = 12322`},
		{`SrcAS=AS12322`, `SrcAS = 12322`},
		{`DstAS=12322`, `DstAS = 12322`},
		{`SrcCountry='FR'`, `SrcCountry = 'FR'`},
		{`DstCountry='FR'`, `DstCountry = 'FR'`},
		{`InIfName='Gi0/0/0/1'`, `InIfName = 'Gi0/0/0/1'`},
		{`OutIfName = 'Gi0/0/0/1'`, `OutIfName = 'Gi0/0/0/1'`},
		{`InIfDescription='Some description'`, `InIfDescription = 'Some description'`},
		{`OutIfDescription='Some other description'`, `OutIfDescription = 'Some other description'`},
		{`InIfSpeed>=1000`, `InIfSpeed >= 1000`},
		{`InIfSpeed!=1000`, `InIfSpeed != 1000`},
		{`InIfSpeed<1000`, `InIfSpeed < 1000`},
		{`OutIfSpeed!=1000`, `OutIfSpeed != 1000`},
		{`InIfConnectivity = 'pni'`, `InIfConnectivity = 'pni'`},
		{`OutIfConnectivity = 'ix'`, `OutIfConnectivity = 'ix'`},
		{`InIfProvider = 'cogent'`, `InIfProvider = 'cogent'`},
		{`OutIfProvider = 'telia'`, `OutIfProvider = 'telia'`},
		{`InIfBoundary = external`, `InIfBoundary = 'external'`},
		{`OutIfBoundary != internal`, `OutIfBoundary != 'internal'`},
		{`EType = ipv4`, `EType = 2048`},
		{`EType != ipv6`, `EType != 34525`},
		{`Proto = 1`, `Proto = 1`},
		{`Proto = 'gre'`, `dictGetOrDefault('protocols', 'name', Proto, '???') = 'gre'`},
		{`SrcPort = 80`, `SrcPort = 80`},
		{`DstPort > 1024`, `DstPort > 1024`},
		{`ForwardingStatus >= 128`, `ForwardingStatus >= 128`},
		{`DstPort > 1024 AND SrcPort < 1024`, `DstPort > 1024 AND SrcPort < 1024`},
		{`DstPort > 1024 OR SrcPort < 1024`, `DstPort > 1024 OR SrcPort < 1024`},
		{`NOT DstPort > 1024 AND SrcPort < 1024`, `NOT DstPort > 1024 AND SrcPort < 1024`},
		{`not DstPort > 1024 and SrcPort < 1024`, `NOT DstPort > 1024 AND SrcPort < 1024`},
		{`DstPort > 1024 AND SrcPort < 1024 OR InIfSpeed >= 1000`,
			`DstPort > 1024 AND SrcPort < 1024 OR InIfSpeed >= 1000`},
		{`DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`,
			`DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`},
		{`  DstPort >   1024   AND   (  SrcPort   <   1024   OR   InIfSpeed   >=   1000   )  `,
			`DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`},
		{`DstPort > 1024 AND(SrcPort < 1024 OR InIfSpeed >= 1000)`,
			`DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`},
		{`DstPort > 1024
                  AND (SrcPort < 1024 OR InIfSpeed >= 1000)`,
			`DstPort > 1024 AND (SrcPort < 1024 OR InIfSpeed >= 1000)`},
		{`(ExporterAddress=203.0.113.1)`, `(ExporterAddress = IPv6StringToNum('203.0.113.1'))`},
	}
	for _, tc := range cases {
		got, err := Parse("", []byte(tc.Input))
		if err != nil {
			t.Errorf("Parse(%q) error:\n%+v", tc.Input, err)
			continue
		}
		if diff := helpers.Diff(got.(string), tc.Output); diff != "" {
			t.Errorf("Parse(%q) (-got, +want):\n%s", tc.Input, diff)
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
		{`ExporterName="something invalid escape \6"`},
		{`ExporterAddress=203.0.113`},
		{`ExporterAddress=2001:db8`},
		{`ExporterAddress="2001:db8:0::1"`},
		{`SrcAS=12322a`},
		{`SrcAS=785473854857857485784`},
		{`EType = ipv7`},
		{`Proto = 1000`},
		{`SrcPort = 1000000`},
		{`ForwardingStatus >= 900`},
		{`Proto = 1000 AND`},
		{`AND Proto = 1000`},
		{`Proto = 1000AND Proto = 1000`},
		{`Proto = 1000 ANDProto = 1000`},
	}
	for _, tc := range cases {
		_, err := Parse("", []byte(tc.Input))
		if err == nil {
			t.Errorf("Parse(%q) didn't throw an error", tc.Input)
		}
	}
}
