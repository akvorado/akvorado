// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

func TestDefaultConfiguration(t *testing.T) {
	config := DefaultConfiguration()
	if err := helpers.Validate.Struct(config); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
	if !config.ManageTopic {
		t.Error("ManageTopic should be true by default")
	}
}

func TestManageTopicDisabled(t *testing.T) {
	config := DefaultConfiguration()
	config.ManageTopic = false
	c, err := New(reporter.NewMock(t), config, Dependencies{Schema: schema.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if c != nil {
		t.Error("Component should be nil when ManageTopic is false")
	}
	helpers.StartStop(t, c)
}

func TestShouldAlterConfiguration(t *testing.T) {
	referenceTestFoo := "foo"
	referenceTestBar := "bar"
	referenceTestOtherFoo := "foo"
	cases := []struct {
		name         string
		target       map[string]*string
		source       map[string]*string
		strictPolicy bool
		expected     bool
	}{
		{
			"subset in strict policy",
			map[string]*string{"a": &referenceTestFoo},
			map[string]*string{"a": &referenceTestFoo, "otherconfigentry": &referenceTestBar},
			true, true,
		}, {
			"subset in non-strict policy",
			map[string]*string{"a": &referenceTestFoo},
			map[string]*string{"a": &referenceTestFoo, "otherconfigentry": &referenceTestBar},
			false, false,
		}, {
			"subset with different references in non strict policy",
			map[string]*string{"a": &referenceTestFoo},
			map[string]*string{"a": &referenceTestOtherFoo, "otherconfigentry": &referenceTestBar},
			false, false,
		}, {
			"missing config entry in strict policy",
			map[string]*string{"a": &referenceTestFoo},
			map[string]*string{"otherconfigentry": &referenceTestBar},
			true, true,
		}, {
			"missing config entry in non-strict policy",
			map[string]*string{"a": &referenceTestFoo},
			map[string]*string{"otherconfigentry": &referenceTestBar},
			false, true,
		}, {
			"same config in strict policy",
			map[string]*string{"a": &referenceTestFoo},
			map[string]*string{"a": &referenceTestOtherFoo},
			true, false,
		}, {
			"same config in non-strict policy",
			map[string]*string{"a": &referenceTestFoo},
			map[string]*string{"a": &referenceTestOtherFoo},
			false, false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldAlterConfiguration(tc.target, tc.source, tc.strictPolicy)
			if got && !tc.expected {
				t.Errorf("Configuration should not update inplace config.")
			} else if !got && tc.expected {
				t.Errorf("Configuration should update inplace config.")
			}
		})
	}
}
