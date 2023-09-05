// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers_test

import (
	"net/netip"
	"testing"

	"akvorado/common/helpers"
)

func TestListenValidator(t *testing.T) {
	s := struct {
		Listen string `validate:"listen"`
	}{}
	cases := []struct {
		Listen string
		Err    bool
	}{
		{"127.0.0.1:161", false},
		{"localhost:161", false},
		{"0.0.0.0:161", false},
		{"0.0.0.0:0", false},
		{":161", false},
		{":0", false},
		{"127.0.0.1:0", false},
		{"localhost", true},
		{"127.0.0.1", true},
		{"127.0.0.1:what", true},
		{"127.0.0.1:100000", true},
	}
	for _, tc := range cases {
		s.Listen = tc.Listen
		err := helpers.Validate.Struct(s)
		if err == nil && tc.Err {
			t.Errorf("Validate.Struct(%q) expected an error", tc.Listen)
		} else if err != nil && !tc.Err {
			t.Errorf("Validate.Struct(%q) error:\n%+v", tc.Listen, err)
		}
	}
}

func TestNoIntersectWithValidator(t *testing.T) {
	s := struct {
		Set1 []string
		Set2 []string `validate:"ninterfield=Set1"`
	}{}
	cases := []struct {
		Set1 []string
		Set2 []string
		Err  bool
	}{
		{nil, nil, false},
		{nil, []string{"aaa"}, false},
		{[]string{}, []string{"bbb"}, false},
		{[]string{}, []string{"bbb"}, false},
		{[]string{"bbb"}, nil, false},
		{[]string{"aaa"}, []string{"bbb"}, false},
		{[]string{"aaa", "ccc"}, []string{"bbb"}, false},
		{[]string{"aaa"}, []string{"aaa"}, true},
		{[]string{"aaa", "ccc"}, []string{"bbb", "ddd"}, false},
		{[]string{"aaa", "ccc"}, []string{"bbb", "ccc"}, true},
		{[]string{"aaa", "ccc"}, []string{"ccc", "bbb"}, true},
	}
	for _, tc := range cases {
		s.Set1 = tc.Set1
		s.Set2 = tc.Set2
		err := helpers.Validate.Struct(s)
		if err == nil && tc.Err {
			t.Errorf("Validate.Struct(%+v) expected an error", s)
		} else if err != nil && !tc.Err {
			t.Errorf("Validate.Struct(%+v) error:\n%+v", s, err)
		}
	}
}

func TestSubnetMapValidator(t *testing.T) {
	type SomeStruct struct {
		Blip *helpers.SubnetMap[string] `validate:"min=2,dive,min=3"`
	}
	type NestedStruct struct {
		First  string `validate:"required"`
		Second string `validate:"oneof=one two"`
	}
	type OtherStruct struct {
		Blip *helpers.SubnetMap[NestedStruct] `validate:"omitempty,dive"`
	}

	helpers.RegisterSubnetMapValidation[NestedStruct]()

	cases := []struct {
		Description string
		Value       interface{}
		Error       bool
	}{
		{
			Description: "Valid SomeStruct",
			Value: SomeStruct{
				Blip: helpers.MustNewSubnetMap(map[string]string{
					"2001:db8::/64":   "hello",
					"2001:db8:1::/64": "bye",
				}),
			},
		}, {
			Description: "Missing one key",
			Value: SomeStruct{
				Blip: helpers.MustNewSubnetMap(map[string]string{
					"2001:db8::/64": "hello",
				}),
			},
			Error: true,
		}, {
			Description: "One value is too short",
			Value: SomeStruct{
				Blip: helpers.MustNewSubnetMap(map[string]string{
					"2001:db8::/64":   "he",
					"2001:db8:1::/64": "bye",
				}),
			},
			Error: true,
		}, {
			Description: "Valid OtherStruct",
			Value: OtherStruct{
				Blip: helpers.MustNewSubnetMap(map[string]NestedStruct{
					"2001:db8::/64": {
						First:  "hello",
						Second: "two",
					},
				}),
			},
		}, {
			Description: "Empty OtherStruct",
			Value: OtherStruct{
				Blip: helpers.MustNewSubnetMap(map[string]NestedStruct{}),
			},
		}, {
			Description: "OtherStruct missing required in NestedStruct",
			Value: OtherStruct{
				Blip: helpers.MustNewSubnetMap(map[string]NestedStruct{
					"2001:db8::/64": {
						Second: "two",
					},
				}),
			},
			Error: true,
		}, {
			Description: "OtherStruct incorrect NestedStruct",
			Value: OtherStruct{
				Blip: helpers.MustNewSubnetMap(map[string]NestedStruct{
					"2001:db8::/64": {
						First:  "hello",
						Second: "three",
					},
				}),
			},
			Error: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			err := helpers.Validate.Struct(tc.Value)
			if err != nil && !tc.Error {
				t.Fatalf("Validate() error:\n%+v", err)
			} else if err == nil && tc.Error {
				t.Fatal("Validate() did not error")
			}
		})
	}
}

func TestNetIPValidation(t *testing.T) {
	type SomeStruct struct {
		Src     netip.Addr   `validate:"required"`
		DstNet  netip.Prefix `validate:"required"`
		Nothing netip.Addr   `validate:"isdefault"`
	}
	cases := []struct {
		Description string
		Value       interface{}
		Error       bool
	}{
		{
			Description: "Valid SomeStruct",
			Value: SomeStruct{
				Src:     netip.MustParseAddr("203.0.113.14"),
				DstNet:  netip.MustParsePrefix("203.0.113.0/24"),
				Nothing: netip.Addr{},
			},
		}, {
			Description: "Missing netip.Addr",
			Value: SomeStruct{
				Src:     netip.Addr{},
				DstNet:  netip.MustParsePrefix("203.0.113.0/24"),
				Nothing: netip.Addr{},
			},
			Error: true,
		}, {
			Description: "Missing netip.Prefix",
			Value: SomeStruct{
				Src:     netip.MustParseAddr("203.0.113.14"),
				DstNet:  netip.Prefix{},
				Nothing: netip.Addr{},
			},
			Error: true,
		}, {
			Description: "Non-default netip.Addr",
			Value: SomeStruct{
				Src:     netip.MustParseAddr("203.0.113.14"),
				DstNet:  netip.MustParsePrefix("203.0.113.0/24"),
				Nothing: netip.MustParseAddr("2001:db8::1"),
			},
			Error: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			err := helpers.Validate.Struct(tc.Value)
			if err != nil && !tc.Error {
				t.Fatalf("Validate() error:\n%+v", err)
			} else if err == nil && tc.Error {
				t.Fatal("Validate() did not error")
			}
		})
	}
}
