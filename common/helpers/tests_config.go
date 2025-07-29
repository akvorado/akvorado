// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package helpers

import (
	"fmt"
	"testing"

	"github.com/go-viper/mapstructure/v2"

	"akvorado/common/helpers/yaml"
)

// ConfigurationDecodeCases describes a test case for configuration
// decode. We use functions to return value as the decoding process
// may mutate the configuration.
type ConfigurationDecodeCases []struct {
	Description    string
	Pos            Pos
	Initial        func() any // initial value for configuration
	Configuration  func() any // configuration to decode
	Expected       any
	Error          bool
	SkipValidation bool
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
				t.Helper()
				var configuration any
				if fromYAML {
					// Encode and decode with YAML
					out, err := yaml.Marshal(tc.Configuration())
					if err != nil {
						t.Fatalf("%syaml.Marshal() error:\n%+v", tc.Pos, err)
					}
					if err := yaml.Unmarshal(out, &configuration); err != nil {
						t.Fatalf("%syaml.Unmarshal() error:\n%+v", tc.Pos, err)
					}
				} else {
					// Just use as is
					configuration = tc.Configuration()
				}
				got := tc.Initial()

				decoder, err := mapstructure.NewDecoder(GetMapStructureDecoderConfig(&got))
				if err != nil {
					t.Fatalf("%sNewDecoder() error:\n%+v", tc.Pos, err)
				}
				err = decoder.Decode(configuration)
				if err != nil && !tc.Error {
					t.Fatalf("%sDecode() error:\n%+v", tc.Pos, err)
				} else if tc.Error && err != nil {
					return
				}

				if !tc.SkipValidation {
					err = Validate.Struct(got)
					if err != nil && !tc.Error {
						t.Fatalf("%sValidate() error:\n%+v", tc.Pos, err)
					} else if tc.Error && err != nil {
						return
					}
					if tc.Error {
						t.Errorf("%sDecode() and Validate() did not error", tc.Pos)
					}
				} else {
					if tc.Error {
						t.Errorf("%sDecode() did not error", tc.Pos)
					}
				}

				if diff := Diff(got, tc.Expected, options...); diff != "" && err == nil {
					t.Fatalf("%sDecode() (-got, +want):\n%s", tc.Pos, diff)
				}
			})
		}
	}
}
