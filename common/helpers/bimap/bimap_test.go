// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bimap_test

import (
	"sort"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/helpers/bimap"
)

func TestBimapLoadValue(t *testing.T) {
	input := bimap.New(map[int]string{
		1: "hello",
		2: "world",
		3: "happy",
	})
	cases := []struct {
		key   int
		value string
		ok    bool
	}{
		{1, "hello", true},
		{2, "world", true},
		{10, "", false},
		{0, "", false},
	}
	for _, tc := range cases {
		got, ok := input.LoadValue(tc.key)
		if ok != tc.ok {
			t.Errorf("LoadValue(%q) ok: %v but expected %v", tc.key, ok, tc.ok)
		}
		if got != tc.value {
			t.Errorf("LoadValue(%q) got: %q but expected %q", tc.key, got, tc.value)
		}
	}
}

func TestBimapLoadKey(t *testing.T) {
	input := bimap.New(map[int]string{
		1: "hello",
		2: "world",
		3: "happy",
	})
	cases := []struct {
		value string
		key   int
		ok    bool
	}{
		{"hello", 1, true},
		{"happy", 3, true},
		{"", 0, false},
		{"nothing", 0, false},
	}
	for _, tc := range cases {
		got, ok := input.LoadKey(tc.value)
		if ok != tc.ok {
			t.Errorf("LoadKey(%q) ok: %v but expected %v", tc.value, ok, tc.ok)
		}
		if got != tc.key {
			t.Errorf("LoadKey(%q) got: %q but expected %q", tc.value, got, tc.value)
		}
	}
}

func TestBimapKeys(t *testing.T) {
	input := bimap.New(map[int]string{
		1: "hello",
		2: "world",
		3: "happy",
	})
	got := input.Keys()
	expected := []int{1, 2, 3}
	sort.Ints(got)
	sort.Ints(expected)
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Keys() (-got, +want):\n%s", diff)
	}
}

func TestBimapValues(t *testing.T) {
	input := bimap.New(map[int]string{
		1: "hello",
		2: "world",
		3: "happy",
	})
	got := input.Values()
	expected := []string{"hello", "world", "happy"}
	sort.Strings(got)
	sort.Strings(expected)
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Values() (-got, +want):\n%s", diff)
	}
}
