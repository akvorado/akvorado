package geoip

import (
	"net"
	"testing"

	"akvorado/helpers"
	"akvorado/reporter"
)

func TestLookup(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r)

	cases := []struct {
		IP       string
		Expected LookupResult
	}{
		{
			IP: "1.0.0.0",
			Expected: LookupResult{
				ASN:          15169,
				Organization: "Google Inc.",
			},
		}, {
			IP: "2.125.160.216",
			Expected: LookupResult{
				Country: "GB",
			},
		}, {
			IP: "2a02:ff00::1:1",
			Expected: LookupResult{
				Country: "IT",
			},
		}, {
			IP: "67.43.156.77",
			Expected: LookupResult{
				ASN:     35908,
				Country: "BT",
			},
		},
	}
	for _, ca := range cases {
		got := c.Lookup(net.ParseIP(ca.IP))
		if diff := helpers.Diff(got, ca.Expected); diff != "" {
			t.Errorf("Lookup(%q) (-got, +want):\n%s", ca.IP, diff)
		}
	}
}
