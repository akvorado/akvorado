// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package yaml_test

import (
	"os"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/helpers/yaml"

	"github.com/gin-gonic/gin"
)

func TestUnmarshalWithIn(t *testing.T) {
	fsys := os.DirFS("testdata")
	var got interface{}
	if err := yaml.UnmarshalWithInclude(fsys, "base.yaml", &got); err != nil {
		t.Fatalf("UnmarshalWithInclude() error:\n%+v", err)
	}
	expected := gin.H{
		"file1": gin.H{"name": "1.yaml"},
		"file2": gin.H{"name": "2.yaml"},
		"nested": gin.H{
			"file1": gin.H{"name": "1.yaml"},
		},
		"list1": []string{"el1", "el2", "el3"},
		"list2": []interface{}{gin.H{
			"protocol": "tcp",
			"size":     1300,
		}, "el2", "el3"},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("UnmarshalWithInclude() (-got, +want):\n%s", diff)
	}
}
