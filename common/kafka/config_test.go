// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"testing"

	"akvorado/common/helpers"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
)

func TestDefaultConfiguration(t *testing.T) {
	if err := helpers.Validate.Struct(DefaultConfiguration()); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
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
				TLS: TLSAndSASLConfiguration{
					TLSConfiguration: helpers.TLSConfiguration{
						Enable: true,
					},
					SASLUsername: "hello",
					SASLPassword: "password",
				},
			},
		}, {
			description: "SASL SCRAM SHA256",
			config: Configuration{
				TLS: TLSAndSASLConfiguration{
					TLSConfiguration: helpers.TLSConfiguration{
						Enable: true,
					},
					SASLUsername:  "hello",
					SASLPassword:  "password",
					SASLMechanism: SASLSCRAMSHA256,
				},
			},
		}, {
			description: "SASL SCRAM SHA512",
			config: Configuration{
				TLS: TLSAndSASLConfiguration{
					TLSConfiguration: helpers.TLSConfiguration{
						Enable: true,
					},
					SASLUsername:  "hello",
					SASLPassword:  "password",
					SASLMechanism: SASLSCRAMSHA512,
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			kafkaConfig, err := NewConfig(tc.config)
			if err != nil {
				t.Fatalf("NewConfig() error:\n%+v", err)
			}
			if err := kafkaConfig.Validate(); err != nil {
				t.Fatalf("Validate() error:\n%+v", err)
			}
		})
	}
}

func TestTLSConfiguration(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description:   "no TLS",
			Initial:       func() interface{} { return DefaultConfiguration() },
			Configuration: func() interface{} { return nil },
			Expected:      DefaultConfiguration(),
		}, {
			Description: "TLS without auth",
			Initial:     func() interface{} { return DefaultConfiguration() },
			Configuration: func() interface{} {
				return gin.H{
					"tls": gin.H{
						"enable": true,
					},
				}
			},
			Expected: Configuration{
				Topic:   "flows",
				Brokers: []string{"127.0.0.1:9092"},
				Version: Version(sarama.V2_8_1_0),
				TLS: TLSAndSASLConfiguration{
					TLSConfiguration: helpers.TLSConfiguration{
						Enable: true,
						Verify: true,
					},
				},
			},
		}, {
			Description: "TLS SASL plain, skip cert verification",
			Initial:     func() interface{} { return DefaultConfiguration() },
			Configuration: func() interface{} {
				return gin.H{
					"tls": gin.H{
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
				Version: Version(sarama.V2_8_1_0),
				TLS: TLSAndSASLConfiguration{
					TLSConfiguration: helpers.TLSConfiguration{
						Enable: true,
						Verify: false,
					},
					SASLUsername:  "hello",
					SASLPassword:  "bye",
					SASLMechanism: SASLPlainText,
				},
			},
		}, {
			Description: "TLS SASL SCRAM 256",
			Initial:     func() interface{} { return DefaultConfiguration() },
			Configuration: func() interface{} {
				return gin.H{
					"tls": gin.H{
						"enable":         true,
						"sasl-username":  "hello",
						"sasl-password":  "bye",
						"sasl-mechanism": "scram-sha256",
					},
				}
			},
			Expected: Configuration{
				Topic:   "flows",
				Brokers: []string{"127.0.0.1:9092"},
				Version: Version(sarama.V2_8_1_0),
				TLS: TLSAndSASLConfiguration{
					TLSConfiguration: helpers.TLSConfiguration{
						Enable: true,
						// Value from DefaultConfig is true
						Verify: true,
					},
					SASLUsername:  "hello",
					SASLPassword:  "bye",
					SASLMechanism: SASLSCRAMSHA256,
				},
			},
		},
	})
}
