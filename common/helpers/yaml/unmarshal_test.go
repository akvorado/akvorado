// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package yaml_test

import (
	"os"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/helpers/yaml"
)

func TestUnmarshalWithIn(t *testing.T) {
	fsys := os.DirFS("testdata")
	var got any
	if err := yaml.UnmarshalWithInclude(fsys, "base.yaml", &got); err != nil {
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
		t.Fatalf("UnmarshalWithInclude() (-got, +want):\n%s", diff)
	}
}
