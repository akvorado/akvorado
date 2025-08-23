// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers_test

import (
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-viper/mapstructure/v2"

	"akvorado/common/helpers/yaml"

	"akvorado/common/helpers"
)

func TestSubnetMapUnmarshalHook(t *testing.T) {
	var nilMap map[string]string
	cases := []struct {
		Pos         helpers.Pos
		Description string
		Input       any
		Tests       map[string]string
		Error       bool
		YAML        any
	}{
		{
			Pos:         helpers.Mark(),
			Description: "nil",
			Input:       nilMap,
			Tests: map[string]string{
				"::ffff:203.0.113.1": "",
			},
			YAML: map[string]string{},
		}, {
			Pos:         helpers.Mark(),
			Description: "empty",
			Input:       map[string]string{},
			Tests: map[string]string{
				"::ffff:203.0.113.1": "",
			},
		}, {
			Pos:         helpers.Mark(),
			Description: "IPv4 subnet",
			Input:       map[string]string{"203.0.113.0/24": "customer1"},
			Tests: map[string]string{
				"::ffff:203.0.113.18": "customer1",
				"::ffff:203.0.113.16": "customer1",
				"203.0.113.16":        "",
				"::ffff:203.0.1.1":    "",
				"203.0.1.1":           "",
				"2001:db8:1::12":      "",
			},
		}, {
			Pos:         helpers.Mark(),
			Description: "IPv4 IP",
			Input:       map[string]string{"203.0.113.1": "customer1"},
			Tests: map[string]string{
				"::ffff:203.0.113.1": "customer1",
				"2001:db8:1::12":     "",
			},
			YAML: map[string]string{"203.0.113.1/32": "customer1"},
		}, {
			Pos:         helpers.Mark(),
			Description: "IPv6 subnet",
			Input:       map[string]string{"2001:db8:1::/64": "customer2"},
			Tests: map[string]string{
				"2001:db8:1::1": "customer2",
				"2001:db8:1::2": "customer2",
				"2001:db8:2::2": "",
			},
		}, {
			Pos:         helpers.Mark(),
			Description: "IPv6-mapped-IPv4 subnet",
			Input:       map[string]string{"::ffff:203.0.113.0/120": "customer2"},
			Tests: map[string]string{
				"::ffff:203.0.113.10": "customer2",
				"::ffff:203.0.112.10": "",
			},
			YAML: map[string]string{"203.0.113.0/24": "customer2"},
		}, {
			Pos:         helpers.Mark(),
			Description: "IPv6 IP",
			Input:       map[string]string{"2001:db8:1::1": "customer2"},
			Tests: map[string]string{
				"2001:db8:1::1": "customer2",
				"2001:db8:1::2": "",
				"2001:db8:2::2": "",
			},
			YAML: map[string]string{"2001:db8:1::1/128": "customer2"},
		}, {
			Pos:         helpers.Mark(),
			Description: "Invalid subnet (1)",
			Input:       map[string]string{"192.0.2.1/38": "customer"},
			Error:       true,
		}, {
			Pos:         helpers.Mark(),
			Description: "Invalid subnet (2)",
			Input:       map[string]string{"192.0.2.1/255.0.255.0": "customer"},
			Error:       true,
		}, {
			Pos:         helpers.Mark(),
			Description: "Invalid subnet (3)",
			Input:       map[string]string{"2001:db8::/1000": "customer"},
			Error:       true,
		}, {
			Pos:         helpers.Mark(),
			Description: "Invalid IP",
			Input:       map[string]string{"200.33.300.1": "customer"},
			Error:       true,
		}, {
			Pos:         helpers.Mark(),
			Description: "Random key",
			Input:       map[string]string{"kfgdjgkfj": "customer"},
			Error:       true,
		}, {
			Pos:         helpers.Mark(),
			Description: "Single value",
			Input:       "customer",
			Tests: map[string]string{
				"::ffff:203.0.113.4": "customer",
				"2001:db8::1":        "customer",
			},
			YAML: map[string]string{
				"::/0": "customer",
			},
		},
	}
	for _, tc := range cases {
		if tc.YAML == nil {
			tc.YAML = tc.Input
		}
		if tc.Tests == nil {
			tc.Tests = map[string]string{}
		}
		t.Run(tc.Description, func(t *testing.T) {
			var tree helpers.SubnetMap[string]
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				Result:      &tree,
				ErrorUnused: true,
				Metadata:    nil,
				DecodeHook:  helpers.SubnetMapUnmarshallerHook[string](),
			})
			if err != nil {
				t.Fatalf("NewDecoder() error:\n%+v", err)
			}
			err = decoder.Decode(tc.Input)
			if err != nil && !tc.Error {
				t.Fatalf("%sDecode() error:\n%+v", tc.Pos, err)
			} else if err == nil && tc.Error {
				t.Fatalf("%sDecode() did not return an error", tc.Pos)
			} else if tc.Error {
				return
			}
			got := map[string]string{}
			for k := range tc.Tests {
				v, _ := tree.Lookup(netip.MustParseAddr(k))
				got[k] = v
			}
			if diff := helpers.Diff(got, tc.Tests); diff != "" {
				t.Fatalf("%sDecode() (-got, +want):\n%s", tc.Pos, diff)
			}

			// Try to unmarshal with YAML
			buf, err := yaml.Marshal(tree)
			if err != nil {
				t.Fatalf("%syaml.Marshal() error:\n%+v", tc.Pos, err)
			}
			got = map[string]string{}
			if err := yaml.Unmarshal(buf, &got); err != nil {
				t.Fatalf("%syaml.Unmarshal() error:\n%+v", tc.Pos, err)
			}
			if diff := helpers.Diff(got, tc.YAML); diff != "" {
				t.Fatalf("%sMarshalYAML() (-got, +want):\n%s", tc.Pos, diff)
			}
		})
	}
}

func TestSubnetMapUnmarshalHookWithMapValue(t *testing.T) {
	type SomeStruct struct {
		Blip string
		Blop string
	}
	cases := []struct {
		Pos      helpers.Pos
		Input    gin.H
		Expected any
	}{
		{
			Pos: helpers.Mark(),
			Input: gin.H{
				"blip": "some",
				"blop": "thing",
			},
			Expected: map[string]SomeStruct{
				"::/0": {
					Blip: "some",
					Blop: "thing",
				},
			},
		}, {
			Pos: helpers.Mark(),
			Input: gin.H{
				"::/0": gin.H{
					"blip": "some",
					"blop": "thing",
				},
				"203.0.113.14": gin.H{
					"blip": "other",
					"blop": "stuff",
				},
			},
			Expected: map[string]SomeStruct{
				"::/0": {
					Blip: "some",
					Blop: "thing",
				},
				"203.0.113.14/32": {
					Blip: "other",
					Blop: "stuff",
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run("sub", func(t *testing.T) {
			var tree helpers.SubnetMap[SomeStruct]
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				Result:      &tree,
				ErrorUnused: true,
				Metadata:    nil,
				DecodeHook:  helpers.SubnetMapUnmarshallerHook[SomeStruct](),
			})
			if err != nil {
				t.Fatalf("%sNewDecoder() error:\n%+v", tc.Pos, err)
			}
			err = decoder.Decode(tc.Input)
			if err != nil {
				t.Fatalf("%sDecode() error:\n%+v", tc.Pos, err)
			}
			if diff := helpers.Diff(tree.ToMap(), tc.Expected); diff != "" {
				t.Fatalf("%sDecode() (-got, +want):\n%s", tc.Pos, diff)
			}
		})
	}
}

func TestSubnetMapParseKey(t *testing.T) {
	cases := []struct {
		Description string
		Input       string
		Expected    string
		Error       bool
	}{
		{
			Description: "valid ipv4 address",
			Input:       "10.0.0.1",
			Expected:    "::ffff:10.0.0.1/128",
		},
		{
			Description: "valid ipv4 subnet",
			Input:       "10.0.0.0/28",
			Expected:    "::ffff:10.0.0.0/124",
		},
		{
			Description: "invalid ipv4 address",
			Input:       "10.0.0",
			Error:       true,
		},
		{
			Description: "valid ipv6 address",
			Input:       "2001:db8:2::a",
			Expected:    "2001:db8:2::a/128",
		},
		{
			Description: "valid ipv6 subnet",
			Input:       "2001:db8:2::/48",
			Expected:    "2001:db8:2::/48",
		},
		{
			Description: "invalid ipv6 address",
			Input:       "2001:",
			Error:       true,
		},
		{
			Description: "invalid string",
			Input:       "foo-bar",
			Error:       true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			res, err := helpers.SubnetMapParseKey(tc.Input)
			if err != nil && !tc.Error {
				t.Fatalf("SubnetMapParseKey() error:\n%+v", err)
			} else if err == nil && tc.Error {
				t.Fatal("SubnetMapParseKey() did not return an error")
			}
			if diff := helpers.Diff(res.String(), tc.Expected); err == nil && diff != "" {
				t.Fatalf("Decode() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestToMap(t *testing.T) {
	input := helpers.MustNewSubnetMap(map[string]string{
		"2001:db8::/64":        "hello",
		"::ffff:192.0.2.0/120": "bye",
	})
	got := input.ToMap()
	expected := map[string]string{
		"2001:db8::/64": "hello",
		"192.0.2.0/24":  "bye",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("ToMap() (-got, +want):\n%s", diff)
	}
}

func TestSubnetMapLookup(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var sm *helpers.SubnetMap[string]
		value, ok := sm.Lookup(netip.MustParseAddr("::ffff:192.0.2.1"))
		if ok || value != "" {
			t.Fatalf("Lookup() == value=%q, ok=%v", value, ok)
		}
	})

	t.Run("empty", func(t *testing.T) {
		sm := &helpers.SubnetMap[string]{}
		value, ok := sm.Lookup(netip.MustParseAddr("::ffff:192.0.2.1"))
		if ok || value != "" {
			t.Fatalf("Lookup() == value=%q, ok=%v", value, ok)
		}
	})

	t.Run("populated", func(t *testing.T) {
		sm := helpers.MustNewSubnetMap(map[string]string{
			"192.0.2.0/24":  "customer1",
			"2001:db8::/64": "customer2",
			"10.0.0.1":      "specific",
		})

		cases := []struct {
			ip       string
			expected string
			found    bool
		}{
			{"::ffff:192.0.2.1", "customer1", true},
			{"::ffff:192.0.2.255", "customer1", true},
			{"::ffff:192.0.3.1", "", false},
			{"::ffff:10.0.0.1", "specific", true},
			{"::ffff:10.0.0.2", "", false},
			{"2001:db8::1", "customer2", true},
			{"2001:db8:1::1", "", false},
		}

		for _, tc := range cases {
			t.Run(tc.ip, func(t *testing.T) {
				value, ok := sm.Lookup(netip.MustParseAddr(tc.ip))
				if ok != tc.found || value != tc.expected {
					t.Fatalf("Lookup(%s) = (%q, %v), want (%q, %v)", tc.ip, value, ok, tc.expected, tc.found)
				}
			})
		}
	})
}

func TestSubnetMapLookupOrDefault(t *testing.T) {
	sm := helpers.MustNewSubnetMap(map[string]string{
		"192.0.2.0/24": "customer1",
	})

	t.Run("found", func(t *testing.T) {
		value := sm.LookupOrDefault(netip.MustParseAddr("::ffff:192.0.2.1"), "default")
		if value != "customer1" {
			t.Fatalf("LookupOrDefault() = %q, want %q", value, "customer1")
		}
	})

	t.Run("not found", func(t *testing.T) {
		value := sm.LookupOrDefault(netip.MustParseAddr("::ffff:192.0.3.1"), "default")
		if value != "default" {
			t.Fatalf("LookupOrDefault() = %q, want %q", value, "default")
		}
	})

	t.Run("nil", func(t *testing.T) {
		var sm *helpers.SubnetMap[string]
		value := sm.LookupOrDefault(netip.MustParseAddr("::ffff:192.0.2.1"), "default")
		if value != "default" {
			t.Fatalf("LookupOrDefault() = %q, want %q", value, "default")
		}
	})
}

func TestSubnetMapSet(t *testing.T) {
	sm := &helpers.SubnetMap[string]{}

	t.Run("set IPv6 subnet", func(t *testing.T) {
		prefix := netip.MustParsePrefix("2001:db8::/64")
		sm.Set(prefix, "test-value")

		value, ok := sm.Lookup(netip.MustParseAddr("2001:db8::1"))
		if !ok || value != "test-value" {
			t.Fatalf("Lookup() = (%q, %v), want (%q, %v)", value, ok, "test-value", true)
		}
	})

	t.Run("set IPv4-mapped IPv6 subnet", func(t *testing.T) {
		prefix := netip.MustParsePrefix("::ffff:192.0.2.0/120")
		sm.Set(prefix, "ipv4-mapped")

		value, ok := sm.Lookup(netip.MustParseAddr("::ffff:192.0.2.1"))
		if !ok || value != "ipv4-mapped" {
			t.Fatalf("Lookup() = (%q, %v), want (%q, %v)", value, ok, "ipv4-mapped", true)
		}
	})

	t.Run("overwrite existing value", func(t *testing.T) {
		prefix := netip.MustParsePrefix("2001:db8::/64")
		sm.Set(prefix, "new-value")

		value, ok := sm.Lookup(netip.MustParseAddr("2001:db8::1"))
		if !ok || value != "new-value" {
			t.Fatalf("Lookup() = (%q, %v), want (%q, %v)", value, ok, "new-value", true)
		}
	})
}

func TestSubnetMapSetPanic(t *testing.T) {
	sm := &helpers.SubnetMap[string]{}

	t.Run("panic on IPv4 subnet", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Set() should panic with IPv4")
			}
		}()
		prefix := netip.MustParsePrefix("192.0.2.0/24")
		sm.Set(prefix, "should-panic")
	})
}

func TestSubnetMapUpdate(t *testing.T) {
	sm := &helpers.SubnetMap[int]{}

	t.Run("update new value", func(t *testing.T) {
		prefix := netip.MustParsePrefix("2001:db8::/64")
		sm.Update(prefix, func(old int, exists bool) int {
			if !exists {
				return 42
			}
			return old + 1
		})

		value, ok := sm.Lookup(netip.MustParseAddr("2001:db8::1"))
		if !ok || value != 42 {
			t.Fatalf("Lookup() = (%d, %v), want (%d, %v)", value, ok, 42, true)
		}
	})

	t.Run("update existing value", func(t *testing.T) {
		prefix := netip.MustParsePrefix("2001:db8::/64")
		sm.Update(prefix, func(old int, exists bool) int {
			if exists {
				return old + 10
			}
			return 0
		})

		value, ok := sm.Lookup(netip.MustParseAddr("2001:db8::1"))
		if !ok || value != 52 {
			t.Fatalf("Lookup() = (%d, %v), want (%d, %v)", value, ok, 52, true)
		}
	})
}

func TestSubnetMapUpdatePanic(t *testing.T) {
	sm := &helpers.SubnetMap[int]{}

	t.Run("panic on IPv4 subnet", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Update() should panic with IPv4")
			}
		}()
		prefix := netip.MustParsePrefix("192.0.2.0/24")
		sm.Update(prefix, func(old int, exists bool) int { return 1 })
	})
}

func TestSubnetMapAll(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var sm *helpers.SubnetMap[string]
		count := 0
		for range sm.All() {
			count++
		}
		if count != 0 {
			t.Fatalf("All() count = %d, want %d", count, 0)
		}
	})

	t.Run("empty", func(t *testing.T) {
		sm := &helpers.SubnetMap[string]{}
		count := 0
		for range sm.All() {
			count++
		}
		if count != 0 {
			t.Fatalf("All() count = %d, want %d", count, 0)
		}
	})

	t.Run("populated", func(t *testing.T) {
		sm := helpers.MustNewSubnetMap(map[string]string{
			"2001:db8::/64":        "ipv6",
			"::ffff:192.0.2.0/120": "ipv4-mapped",
		})

		items := make(map[string]string)
		for prefix, value := range sm.All() {
			items[prefix.String()] = value
		}

		expected := map[string]string{
			"2001:db8::/64":        "ipv6",
			"::ffff:192.0.2.0/120": "ipv4-mapped",
		}

		if diff := helpers.Diff(items, expected); diff != "" {
			t.Fatalf("All() (-got, +want):\n%s", diff)
		}
	})
}

func TestSubnetMapString(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		sm := &helpers.SubnetMap[string]{}
		str := sm.String()
		expected := "map[]"
		if str != expected {
			t.Fatalf("String() = %q, want %q", str, expected)
		}
	})

	t.Run("populated", func(t *testing.T) {
		sm := helpers.MustNewSubnetMap(map[string]string{
			"192.0.2.0/24": "customer",
		})
		str := sm.String()
		if diff := helpers.Diff(str, "map[192.0.2.0/24:customer]"); diff != "" {
			t.Fatalf("String() (-got, +want):\n%s", diff)
		}
	})
}

func TestPrefixTo16(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "IPv4 prefix",
			input:    "192.0.2.0/24",
			expected: "::ffff:192.0.2.0/120",
		},
		{
			name:     "IPv4 host",
			input:    "192.0.2.1/32",
			expected: "::ffff:192.0.2.1/128",
		},
		{
			name:     "IPv6 prefix unchanged",
			input:    "2001:db8::/64",
			expected: "2001:db8::/64",
		},
		{
			name:     "IPv6 host unchanged",
			input:    "2001:db8::1/128",
			expected: "2001:db8::1/128",
		},
		{
			name:     "IPv4-mapped IPv6 unchanged",
			input:    "::ffff:192.0.2.0/120",
			expected: "::ffff:192.0.2.0/120",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prefix := netip.MustParsePrefix(tc.input)
			result := helpers.PrefixTo16(prefix)
			if result.String() != tc.expected {
				t.Fatalf("PrefixTo16(%s) = %s, want %s", tc.input, result, tc.expected)
			}
		})
	}
}

func TestSubnetMapSupernets(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var sm *helpers.SubnetMap[string]
		count := 0
		for range sm.Supernets(netip.MustParsePrefix("::ffff:192.0.2.1/128")) {
			count++
		}
		if count != 0 {
			t.Fatalf("Supernets() count = %d, want %d", count, 0)
		}
	})

	t.Run("empty", func(t *testing.T) {
		sm := &helpers.SubnetMap[string]{}
		count := 0
		for range sm.Supernets(netip.MustParsePrefix("::ffff:192.0.2.1/128")) {
			count++
		}
		if count != 0 {
			t.Fatalf("Supernets() count = %d, want %d", count, 0)
		}
	})

	t.Run("hierarchical supernets", func(t *testing.T) {
		sm := helpers.MustNewSubnetMap(map[string]string{
			"192.0.0.0/16": "region",
			"192.0.2.0/24": "site",
			"192.0.2.0/28": "rack",
		})

		// Query for 192.0.2.1/32 should find supernets in reverse-CIDR order
		var results []string
		var prefixes []string
		for prefix, value := range sm.Supernets(netip.MustParsePrefix("::ffff:192.0.2.1/128")) {
			results = append(results, value)
			prefixes = append(prefixes, prefix.String())
		}

		expectedValues := []string{"rack", "site", "region"}
		expectedPrefixes := []string{"::ffff:192.0.2.0/124", "::ffff:192.0.2.0/120", "::ffff:192.0.0.0/112"}

		if diff := helpers.Diff(results, expectedValues); diff != "" {
			t.Errorf("Supernets() values (-got, +want):\n%s", diff)
		}
		if diff := helpers.Diff(prefixes, expectedPrefixes); diff != "" {
			t.Errorf("Supernets() prefixes (-got, +want):\n%s", diff)
		}

		count := 0
		for range sm.Supernets(netip.MustParsePrefix("::ffff:10.0.0.1/128")) {
			count++
		}
		if count != 0 {
			t.Fatalf("Supernets() count = %d, want %d", count, 0)
		}
	})
}

func TestSubnetMapAllMaybeSorted(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var sm *helpers.SubnetMap[string]
		count := 0
		for range sm.AllMaybeSorted() {
			count++
		}
		if count != 0 {
			t.Fatalf("AllMaybeSorted() count = %d, want %d", count, 0)
		}
	})

	t.Run("empty", func(t *testing.T) {
		sm := &helpers.SubnetMap[string]{}
		count := 0
		for range sm.AllMaybeSorted() {
			count++
		}
		if count != 0 {
			t.Fatalf("AllMaybeSorted() count = %d, want %d", count, 0)
		}
	})

	t.Run("sorted vs unsorted", func(t *testing.T) {
		sm := helpers.MustNewSubnetMap(map[string]string{
			"192.0.2.0/28":    "rack",
			"192.0.0.0/16":    "region",
			"192.0.2.0/24":    "site",
			"2001:db8:1::/64": "ipv6-site1",
			"2001:db8::/32":   "ipv6-region",
			"2001:db8:2::/64": "ipv6-site2",
		})

		// Collect results from All() (potentially unsorted)
		var allPrefixes []string
		for prefix := range sm.All() {
			allPrefixes = append(allPrefixes, prefix.String())
		}

		// Collect results from AllMaybeSorted() (sorted during tests)
		var sortedPrefixes []string
		for prefix := range sm.AllMaybeSorted() {
			sortedPrefixes = append(sortedPrefixes, prefix.String())
		}

		// Expected sorted order: IPv6 addresses first (sorted), then IPv4-mapped IPv6 (sorted)
		expectedPrefixes := []string{
			"::ffff:192.0.0.0/112",
			"::ffff:192.0.2.0/120",
			"::ffff:192.0.2.0/124",
			"2001:db8::/32",
			"2001:db8:1::/64",
			"2001:db8:2::/64",
		}

		// AllMaybeSorted() should be sorted during tests
		if diff := helpers.Diff(sortedPrefixes, expectedPrefixes); diff != "" {
			t.Errorf("AllMaybeSorted() prefixes (-got, +want):\n%s", diff)
		}

		// All() and AllMaybeSorted() should contain the same elements (but potentially different order)
		if len(allPrefixes) != len(sortedPrefixes) {
			t.Errorf("All() returned %d prefixes, AllMaybeSorted() returned %d", len(allPrefixes), len(sortedPrefixes))
		}

		// Verify that All() and AllMaybeSorted() return different orders (otherwise test is meaningless)
		if slices.Equal(allPrefixes, sortedPrefixes) {
			t.Skip("All() and AllMaybeSorted() returned identical order")
		}
	})
}

func TestSubnetmapDiff(t *testing.T) {
	got := helpers.MustNewSubnetMap(map[string]string{
		"2001:db8::/64":        "hello",
		"::ffff:192.0.2.0/120": "bye",
	})
	expected := helpers.MustNewSubnetMap(map[string]string{
		"2001:db8::/64":        "hello",
		"::ffff:192.0.2.0/120": "bye",
	})

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Diff():\n%+v", diff)
	}

	got.Set(netip.MustParsePrefix("2001:db8:1::/64"), "bye")
	diffGot := helpers.Diff(got, expected)
	diffGot = strings.ReplaceAll(diffGot, "\u00a0", " ")
	diffExpected := `  (*helpers.SubnetMap[string])(Inverse(subnetmap.Transform, map[string]string{
  	"192.0.2.0/24":    "bye",
- 	"2001:db8:1::/64": "bye",
  	"2001:db8::/64":   "hello",
  }))
`
	if diff := helpers.Diff(diffGot, diffExpected); diff != "" {
		t.Fatalf("Diff() (-got, +want):\n%+v", diff)
	}
}
