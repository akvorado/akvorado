package geoip

import (
	"net"
	"testing"

	"akvorado/common/reporter"
)

func TestIterDatabase(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, true)

	mustHave := []struct {
		IP              string
		ExpectedASN     uint32
		ExpectedCountry string
		hasCountry      bool
		hasASN          bool
	}{
		// ipinfo database
		{
			IP:              "2.19.4.138",
			ExpectedASN:     32787,
			ExpectedCountry: "SG",
		}, {
			IP:              "2a09:bac1:14a0:fd0::a:1",
			ExpectedASN:     13335,
			ExpectedCountry: "CA",
		}, {
			IP:              "213.248.218.137",
			ExpectedASN:     43519,
			ExpectedCountry: "HK",
		},
		// maxmind
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

	for _, asnDb := range c.config.ASNDatabase {
		err := c.IterASNDatabase(asnDb, func(n *net.IPNet, a ASNInfo) error {
			for i, h := range mustHave {
				// found the IP
				if n.Contains(net.ParseIP(h.IP)) {
					if h.ExpectedASN != 0 && a.ASNumber != h.ExpectedASN {
						t.Errorf("expected ASN %d, got %d", h.ExpectedASN, a.ASNumber)
					}
					mustHave[i].hasASN = true
					break
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, geoDb := range c.config.GeoDatabase {
		err := c.IterGeoDatabase(geoDb, func(n *net.IPNet, a GeoInfo) error {
			for i, h := range mustHave {
				// found the IP
				if n.Contains(net.ParseIP(h.IP).To16()) {
					if h.ExpectedCountry != "" && a.Country != h.ExpectedCountry {
						t.Errorf("expected Country %s, got %s", h.ExpectedCountry, a.Country)
					}
					mustHave[i].hasCountry = true
					break
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, h := range mustHave {
		if !h.hasASN && h.ExpectedASN != 0 {
			t.Errorf("missing subnet %s in ASN database", h.IP)
		}
		if !h.hasCountry && h.ExpectedCountry != "" {
			t.Errorf("missing subnet %s in GEO database", h.IP)
		}
	}
}
