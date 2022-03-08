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
	for _, ca := range cases {
		gotCountry := c.LookupCountry(net.ParseIP(ca.IP))
		if diff := helpers.Diff(gotCountry, ca.ExpectedCountry); diff != "" {
			t.Errorf("LookupCountry(%q) (-got, +want):\n%s", ca.IP, diff)
		}
		gotASN := c.LookupASN(net.ParseIP(ca.IP))
		if diff := helpers.Diff(gotASN, ca.ExpectedASN); diff != "" {
			t.Errorf("LookupASN(%q) (-got, +want):\n%s", ca.IP, diff)
		}
	}
}
