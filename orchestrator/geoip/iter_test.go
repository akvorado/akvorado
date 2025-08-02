// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"net/netip"
	"path/filepath"
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestIterDatabase(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, true)

	mustHave := []struct {
		IP              string
		ExpectedASN     uint32
		ExpectedCountry string
		ExpectedState   string
		ExpectedCity    string
		hasCountry      bool
		hasASN          bool
	}{
		// ipinfo database
		{
			IP:              "1.0.84.10",
			ExpectedCountry: "JP",
			ExpectedState:   "Shimane",
			ExpectedCity:    "Matsue",
		}, {
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
			ExpectedState:   "ENG",
			ExpectedCity:    "Boxford",
		}, {
			IP:              "2a02:ff00::1:1",
			ExpectedCountry: "IT",
		}, {
			IP:              "67.43.156.77",
			ExpectedASN:     35908,
			ExpectedCountry: "BT",
		},
	}

	err := c.IterASNDatabases(func(prefix netip.Prefix, a ASNInfo) error {
		for i, h := range mustHave {
			// found the IP
			if ip, err := netip.ParseAddr(h.IP); err == nil && prefix.Contains(ip) {
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
		t.Fatalf("IterASNDatabases() error:\n%+v", err)
	}

	err = c.IterGeoDatabases(func(prefix netip.Prefix, a GeoInfo) error {
		for i, h := range mustHave {
			// found the IP
			if ip, err := netip.ParseAddr(h.IP); err == nil && prefix.Contains(ip) {
				if h.ExpectedCountry != "" && a.Country != h.ExpectedCountry {
					t.Errorf("expected Country %s, got %s", h.ExpectedCountry, a.Country)
				}
				if h.ExpectedState != "" && a.State != h.ExpectedState {
					t.Errorf("expected State %s, got %s", h.ExpectedState, a.State)
				}
				if h.ExpectedCity != "" && a.City != h.ExpectedCity {
					t.Errorf("expected City %s, got %s", h.ExpectedCity, a.City)
				}
				mustHave[i].hasCountry = true
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("IterGeoDatabases() error:\n%+v", err)
	}

	for _, h := range mustHave {
		if !h.hasASN && h.ExpectedASN != 0 {
			t.Errorf("missing subnet %s in ASN database", h.IP)
		}
		if !h.hasCountry && h.ExpectedCountry != "" {
			t.Errorf("missing subnet %s in geo database", h.IP)
		}
	}
}

func TestIterNonExistingDatabase(t *testing.T) {
	dir := t.TempDir()
	config := DefaultConfiguration()
	config.GeoDatabase = append(config.GeoDatabase, filepath.Join(dir, "1.mmdb"))
	config.ASNDatabase = append(config.ASNDatabase, filepath.Join(dir, "2.mmdb"))
	config.Optional = true

	r := reporter.NewMock(t)
	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)
	if err := c.IterASNDatabases(func(_ netip.Prefix, _ ASNInfo) error {
		return nil
	}); err != nil {
		t.Fatalf("IterASNDatabases() error:\n%+v", err)
	}
	if err := c.IterGeoDatabases(func(_ netip.Prefix, _ GeoInfo) error {
		return nil
	}); err != nil {
		t.Fatalf("IterGeoDatabases() error:\n%+v", err)
	}
}
