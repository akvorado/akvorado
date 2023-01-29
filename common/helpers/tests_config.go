// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package helpers

import (
	"fmt"
	"testing"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
)

// ConfigurationDecodeCases describes a test case for configuration
// decode. We use functions to return value as the decoding process
// may mutate the configuration.
type ConfigurationDecodeCases []struct {
	Description   string
	Initial       func() interface{} // initial value for configuration
	Configuration func() interface{} // configuration to decode
	Expected      interface{}
	Error         bool
}

// TestConfigurationDecode helps decoding configuration. It also test decoding from YAML.
func TestConfigurationDecode(t *testing.T, cases ConfigurationDecodeCases, options ...DiffOption) {
	t.Helper()
	for _, tc := range cases {
		for _, fromYAML := range []bool{false, true} {
			title := tc.Description
			if fromYAML {
				title = fmt.Sprintf("%s (from YAML)", title)
				if tc.Configuration == nil {
					continue
				}
			}
			t.Run(title, func(t *testing.T) {
				var configuration interface{}
				if fromYAML {
					// Encode and decode with YAML
					out, err := yaml.Marshal(tc.Configuration())
					if err != nil {
						t.Fatalf("yaml.Marshal() error:\n%+v", err)
					}
					if err := yaml.Unmarshal(out, &configuration); err != nil {
						t.Fatalf("yaml.Unmarshal() error:\n%+v", err)
					}
				} else {
					// Just use as is
					configuration = tc.Configuration()
				}
				got := tc.Initial()

				decoder, err := mapstructure.NewDecoder(GetMapStructureDecoderConfig(&got))
				if err != nil {
					t.Fatalf("NewDecoder() error:\n%+v", err)
				}
				err = decoder.Decode(configuration)
				if err != nil && !tc.Error {
					t.Fatalf("Decode() error:\n%+v", err)
				} else if err == nil && tc.Error {
					t.Errorf("Decode() did not error")
				}

				if diff := Diff(got, tc.Expected, options...); diff != "" && err == nil {
					t.Fatalf("Decode() (-got, +want):\n%s", diff)
				}
			})
		}
	}
}
