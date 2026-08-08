// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema_test

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/schema"
)

func TestCustomDictLayoutDecode(t *testing.T) {
	// An empty layout means the key is absent from the configuration.
	customDict := func(layout string) func() any {
		return func() any {
			dict := helpers.M{
				"keys":       []helpers.M{{"name": "addr"}},
				"attributes": []helpers.M{{"name": "role"}},
				"source":     "test.csv",
				"dimensions": []string{"SrcAddr"},
			}
			if layout != "" {
				dict["layout"] = layout
			}
			return helpers.M{"custom-dictionaries": helpers.M{"test": dict}}
		}
	}
	expected := func(layout schema.CustomDictLayout) schema.Configuration {
		config := schema.DefaultConfiguration()
		config.CustomDictionaries = map[string]schema.CustomDict{
			"test": {
				Layout:     layout,
				Keys:       []schema.CustomDictKey{{Name: "addr", Type: "String"}},
				Attributes: []schema.CustomDictAttribute{{Name: "role", Type: "String"}},
				Source:     "test.csv",
				Dimensions: []string{"SrcAddr"},
			},
		}
		return config
	}
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Pos:           helpers.Mark(),
			Description:   "default layout",
			Initial:       func() any { return schema.DefaultConfiguration() },
			Configuration: customDict(""),
			Expected:      expected("hashed"),
		}, {
			Pos:           helpers.Mark(),
			Description:   "complex_key_hashed layout",
			Initial:       func() any { return schema.DefaultConfiguration() },
			Configuration: customDict("complex_key_hashed"),
			Expected:      expected("complex_key_hashed"),
		}, {
			Pos:           helpers.Mark(),
			Description:   "ip_trie layout",
			Initial:       func() any { return schema.DefaultConfiguration() },
			Configuration: customDict("ip_trie"),
			Expected:      expected("ip_trie"),
		}, {
			Pos:           helpers.Mark(),
			Description:   "iptrie layout (compatibility)",
			Initial:       func() any { return schema.DefaultConfiguration() },
			Configuration: customDict("iptrie"),
			Expected:      expected("ip_trie"),
		}, {
			Pos:           helpers.Mark(),
			Description:   "unknown layout",
			Initial:       func() any { return schema.DefaultConfiguration() },
			Configuration: customDict("range_hashed"),
			Error:         true,
		},
	})
}
