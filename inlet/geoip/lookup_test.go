package geoip

import (
	"net"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestLookup(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r)

	cases := []struct {
		IP              string
		ExpectedASN     uint32
		ExpectedCountry string
	}{
		{
			IP:          "1.0.0.0",
			ExpectedASN: 15169,
		}, {
			IP:              "2.125.160.216",
			ExpectedCountry: "GB",
		}, {
			IP:              "2a02:ff00::1:1",
			ExpectedCountry: "IT",
		}, {
			IP:              "67.43.156.77",
			ExpectedASN:     35908,
			ExpectedCountry: "BT",
		},
	}
	for _, tc := range cases {
		gotCountry := c.LookupCountry(net.ParseIP(tc.IP))
		if diff := helpers.Diff(gotCountry, tc.ExpectedCountry); diff != "" {
			t.Errorf("LookupCountry(%q) (-got, +want):\n%s", tc.IP, diff)
		}
		gotASN := c.LookupASN(net.ParseIP(tc.IP))
		if diff := helpers.Diff(gotASN, tc.ExpectedASN); diff != "" {
			t.Errorf("LookupASN(%q) (-got, +want):\n%s", tc.IP, diff)
		}
	}
}
