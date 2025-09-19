// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package yaml_test

import (
	"os"
	"slices"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/helpers/yaml"
)

func TestUnmarshalWithIn(t *testing.T) {
	fsys := os.DirFS("testdata")
	var got any
	gotPaths, err := yaml.UnmarshalWithInclude(fsys, "base.yaml", &got)
	if err != nil {
		t.Fatalf("UnmarshalWithInclude() error:\n%+v", err)
	}
	expected := map[string]any{
		"file1": map[string]any{"name": "1.yaml"},
		"file2": map[string]any{"name": "2.yaml"},
		"nested": map[string]any{
			"file1": map[string]any{"name": "1.yaml"},
		},
		"list1": []any{"el1", "el2", "el3"},
		"list2": []any{map[string]any{
			"protocol": "tcp",
			"size":     1300,
		}, "el2", "el3"},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("UnmarshalWithInclude() (-got, +want):\n%s", diff)
	}
	expectedPaths := []string{
		"base.yaml",
		"1.yaml",
		"2.yaml",
		"nested.yaml",
		"list1.yaml",
		"list2.yaml",
	}
	slices.Sort(expectedPaths)
	slices.Sort(gotPaths)
	if diff := helpers.Diff(gotPaths, expectedPaths); diff != "" {
		t.Errorf("UnmarshalWithInclude() paths (-got, +want):\n%s", diff)
	}
}
