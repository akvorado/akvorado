// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestDefaultConfiguration(t *testing.T) {
	if err := helpers.Validate.Struct(DefaultConfiguration()); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}

func TestCompressionCodecUnmarshal(t *testing.T) {
	cases := []struct {
		Input         string
		Expected      kgo.CompressionCodec
		ExpectedError bool
	}{
		{"none", kgo.NoCompression(), false},
		{"zstd", kgo.ZstdCompression(), false},
		{"gzip", kgo.GzipCompression(), false},
		{"snappy", kgo.SnappyCompression(), false},
		{"lz4", kgo.Lz4Compression(), false},
		{"unknown", kgo.NoCompression(), true},
	}
	for _, tc := range cases {
		var cmp CompressionCodec
		err := cmp.UnmarshalText([]byte(tc.Input))
		if err != nil && !tc.ExpectedError {
			t.Errorf("UnmarshalText(%q) error:\n%+v", tc.Input, err)
			continue
		}
		if err == nil && tc.ExpectedError {
			t.Errorf("UnmarshalText(%q) got %v but expected error", tc.Input, cmp)
			continue
		}
		if !tc.ExpectedError && cmp != CompressionCodec(tc.Expected) {
			t.Errorf("UnmarshalText(%q) got %v but expected %v", tc.Input, cmp, tc.Expected)
			continue
		}
		if !tc.ExpectedError && cmp.String() != tc.Input {
			t.Errorf("String() got %q but expected %q", cmp.String(), tc.Input)
		}
	}
}

func TestKafkaNewConfig(t *testing.T) {
	// It is a bit a pain to test the result, just check we don't have an error
	cases := []struct {
		description string
		config      Configuration
	}{
		{
			description: "No TLS",
			config:      DefaultConfiguration(),
		}, {
			description: "SASL plain",
			config: Configuration{
				TLS: helpers.TLSConfiguration{
					Enable: true,
				},
				SASL: SASLConfiguration{
					Username:  "hello",
					Password:  "password",
					Mechanism: SASLPlain,
				},
			},
		}, {
			description: "SASL SCRAM SHA256",
			config: Configuration{
				TLS: helpers.TLSConfiguration{
					Enable: true,
				},
				SASL: SASLConfiguration{
					Username:  "hello",
					Password:  "password",
					Mechanism: SASLScramSHA256,
				},
			},
		}, {
			description: "SASL SCRAM SHA512",
			config: Configuration{
				TLS: helpers.TLSConfiguration{
					Enable: true,
				},
				SASL: SASLConfiguration{
					Username:  "hello",
					Password:  "password",
					Mechanism: SASLScramSHA512,
				},
			},
		}, {
			description: "SASL OAuth2",
			config: Configuration{
				TLS: helpers.TLSConfiguration{
					Enable: true,
				},
				SASL: SASLConfiguration{
					Username:      "hello",
					Password:      "password",
					Mechanism:     SASLOauth,
					OAuthTokenURL: "http://example.com/token",
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			r := reporter.NewMock(t)
			_, err := NewConfig(r, tc.config)
			if err != nil {
				t.Fatalf("NewConfig() error:\n%+v", err)
			}
		})
	}
}

func TestTLSConfiguration(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description:   "no TLS",
			Initial:       func() any { return DefaultConfiguration() },
			Configuration: func() any { return nil },
			Expected:      DefaultConfiguration(),
		}, {
			Description: "TLS without auth",
			Initial:     func() any { return DefaultConfiguration() },
			Configuration: func() any {
				return helpers.M{
					"tls": helpers.M{
						"enable": true,
					},
				}
			},
			Expected: Configuration{
				Topic:   "flows",
				Brokers: []string{"127.0.0.1:9092"},
				TLS: helpers.TLSConfiguration{
					Enable:     true,
					SkipVerify: false,
				},
			},
		}, {
			Description: "TLS SASL plain, skip cert verification (old style)",
			Initial:     func() any { return DefaultConfiguration() },
			Configuration: func() any {
				return helpers.M{
					"tls": helpers.M{
						"enable":         true,
						"verify":         false,
						"sasl-username":  "hello",
						"sasl-password":  "bye",
						"sasl-mechanism": "plain",
					},
				}
			},
			Expected: Configuration{
				Topic:   "flows",
				Brokers: []string{"127.0.0.1:9092"},
				TLS: helpers.TLSConfiguration{
					Enable:     true,
					SkipVerify: true,
				},
				SASL: SASLConfiguration{
					Username:  "hello",
					Password:  "bye",
					Mechanism: SASLPlain,
				},
			},
		}, {
			Description: "TLS SASL plain, skip cert verification",
			Initial:     func() any { return DefaultConfiguration() },
			Configuration: func() any {
				return helpers.M{
					"sasl": helpers.M{
						"username":  "hello",
						"password":  "bye",
						"mechanism": "plain",
					},
				}
			},
			Expected: Configuration{
				Topic:   "flows",
				Brokers: []string{"127.0.0.1:9092"},
				TLS: helpers.TLSConfiguration{
					Enable:     false,
					SkipVerify: false,
				},
				SASL: SASLConfiguration{
					Username:  "hello",
					Password:  "bye",
					Mechanism: SASLPlain,
				},
			},
		}, {
			Description: "TLS SASL SCRAM 256",
			Initial:     func() any { return DefaultConfiguration() },
			Configuration: func() any {
				return helpers.M{
					"tls": helpers.M{
						"enable": true,
					},
					"sasl": helpers.M{
						"username":  "hello",
						"password":  "bye",
						"mechanism": "scram-sha256",
					},
				}
			},
			Expected: Configuration{
				Topic:   "flows",
				Brokers: []string{"127.0.0.1:9092"},
				TLS: helpers.TLSConfiguration{
					Enable: true,
					// Value from DefaultConfig is true
					SkipVerify: false,
				},
				SASL: SASLConfiguration{
					Username:  "hello",
					Password:  "bye",
					Mechanism: SASLScramSHA256,
				},
			},
		}, {
			Description: "TLS SASL OAuth",
			Initial:     func() any { return DefaultConfiguration() },
			Configuration: func() any {
				return helpers.M{
					"tls": helpers.M{
						"enable": true,
					},
					"sasl": helpers.M{
						"username":        "hello",
						"password":        "bye",
						"mechanism":       "oauth",
						"oauth-token-url": "http://example.com/token",
						"oauth-scopes":    "one,two",
					},
				}
			},
			Expected: Configuration{
				Topic:   "flows",
				Brokers: []string{"127.0.0.1:9092"},
				TLS: helpers.TLSConfiguration{
					Enable: true,
					// Value from DefaultConfig is true
					SkipVerify: false,
				},
				SASL: SASLConfiguration{
					Username:      "hello",
					Password:      "bye",
					Mechanism:     SASLOauth,
					OAuthTokenURL: "http://example.com/token",
					OAuthScopes:   []string{"one", "two"},
				},
			},
		}, {
			Description: "OAuth requires a token URL",
			Initial:     func() any { return DefaultConfiguration() },
			Configuration: func() any {
				return helpers.M{
					"sasl": helpers.M{
						"username":  "hello",
						"password":  "bye",
						"mechanism": "oauth",
					},
				}
			},
			Error: true,
		}, {
			Description: "OAuth token URL only with OAuth",
			Initial:     func() any { return DefaultConfiguration() },
			Configuration: func() any {
				return helpers.M{
					"sasl": helpers.M{
						"username":        "hello",
						"password":        "bye",
						"mechanism":       "plain",
						"oauth-token-url": "http://example.com/token",
					},
				}
			},
			Error: true,
		},
	})
}
