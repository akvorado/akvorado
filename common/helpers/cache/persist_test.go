// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cache_test

import (
	"errors"
	"io/fs"
	"net/netip"
	"path/filepath"
	"testing"
	"time"

	"akvorado/common/helpers/cache"
)

func TestLoadNotExist(t *testing.T) {
	c := cache.New[netip.Addr, string]()
	err := c.Load("/i/do/not/exist")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("c.Load() error:\n%s", err)
	}
}

func TestSaveLoad(t *testing.T) {
	c := cache.New[netip.Addr, string]()
	t1 := time.Date(2022, time.December, 31, 10, 23, 0, 0, time.UTC)
	t2 := t1.Add(time.Minute)
	t3 := t2.Add(time.Minute)
	c.Put(t1, netip.MustParseAddr("::ffff:127.0.0.1"), "entry1")
	c.Put(t2, netip.MustParseAddr("::ffff:127.0.0.2"), "entry2")
	c.Put(t3, netip.MustParseAddr("::ffff:127.0.0.3"), "entry3")

	target := filepath.Join(t.TempDir(), "cache")
	if err := c.Save(target); err != nil {
		t.Fatalf("c.Save() error:\n%s", err)
	}

	c = cache.New[netip.Addr, string]()
	if err := c.Load(target); err != nil {
		t.Fatalf("c.Load() error:\n%s", err)
	}

	expectCacheGet(t, c, "127.0.0.1", "entry1", true)
	expectCacheGet(t, c, "127.0.0.2", "entry2", true)
	expectCacheGet(t, c, "127.0.0.3", "entry3", true)
}

func TestLoadMismatchVersion(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		c1 := cache.New[netip.Addr, string]()
		c1.Put(time.Now(), netip.MustParseAddr("::ffff:127.0.0.1"), "entry1")
		target := filepath.Join(t.TempDir(), "cache")

		if err := c1.Save(target); err != nil {
			t.Fatalf("c.Save() error:\n%s", err)
		}

		// Try to load it
		c2 := cache.New[netip.Addr, int]()
		if err := c2.Load(target); !errors.Is(err, cache.ErrVersion) {
			t.Fatalf("c.Load() error:\n%s", err)
		}
	})
	t.Run("structs", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "cache")
		{
			type Interface struct {
				Name        string
				Description string
				Speed       uint
			}
			type Answer struct {
				ExporterName string
				Interface
			}

			c1 := cache.New[string, Answer]()
			c1.Put(time.Now(),
				"test1",
				Answer{
					ExporterName: "exporter1",
					Interface: Interface{
						Name:        "ifname1",
						Description: "first",
						Speed:       1000,
					},
				})

			if err := c1.Save(target); err != nil {
				t.Fatalf("c.Save() error:\n%s", err)
			}
		}

		// Try to load it
		{
			type Exporter struct {
				Name   string
				Region string
				Role   string
				Tenant string
				Site   string
				Group  string
			}
			type Interface struct {
				Name         string
				Description  string
				Speed        uint
				Provider     string
				Connectivity string
			}
			type Answer struct {
				Exporter  Exporter
				Interface Interface
			}
			c2 := cache.New[string, Answer]()
			if err := c2.Load(target); !errors.Is(err, cache.ErrVersion) {
				t.Fatalf("c.Load() error:\n%s", err)
			}
		}
	})
}
