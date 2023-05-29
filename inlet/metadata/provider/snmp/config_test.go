// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"testing"

	"akvorado/common/helpers"

	"github.com/gin-gonic/gin"
	"github.com/gosnmp/gosnmp"
)

func TestDefaultConfiguration(t *testing.T) {
	if err := helpers.Validate.Struct(DefaultConfiguration()); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}

func TestConfigurationUnmarshallerHook(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description:   "nil",
			Initial:       func() interface{} { return Configuration{} },
			Configuration: func() interface{} { return nil },
			Expected:      Configuration{},
		}, {
			Description:   "empty",
			Initial:       func() interface{} { return Configuration{} },
			Configuration: func() interface{} { return gin.H{} },
			Expected: Configuration{
				Communities: helpers.MustNewSubnetMap(map[string]string{
					"::/0": "public",
				}),
			},
		}, {
			Description: "no communities, no default community",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-retries": 10,
				}
			},
			Expected: Configuration{
				PollerRetries: 10,
				Communities: helpers.MustNewSubnetMap(map[string]string{
					"::/0": "public",
				}),
			},
		}, {
			Description: "communities, no default community",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"communities": gin.H{
						"203.0.113.0/25":   "public",
						"203.0.113.128/25": "private",
					},
				}
			},
			Expected: Configuration{
				Communities: helpers.MustNewSubnetMap(map[string]string{
					"::/0":                     "public",
					"::ffff:203.0.113.0/121":   "public",
					"::ffff:203.0.113.128/121": "private",
				}),
			},
		}, {
			Description: "no communities, default community",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"default-community": "private",
				}
			},
			Expected: Configuration{
				Communities: helpers.MustNewSubnetMap(map[string]string{
					"::/0": "private",
				}),
			},
		}, {
			Description: "communities, default community",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"default-community": "private",
					"communities": gin.H{
						"203.0.113.0/25":   "public",
						"203.0.113.128/25": "private",
					},
				}
			},
			Expected: Configuration{
				Communities: helpers.MustNewSubnetMap(map[string]string{
					"::/0":                     "private",
					"::ffff:203.0.113.0/121":   "public",
					"::ffff:203.0.113.128/121": "private",
				}),
			},
		}, {
			Description: "communities as a string",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"communities": "private",
				}
			},
			Expected: Configuration{
				Communities: helpers.MustNewSubnetMap(map[string]string{
					"::/0": "private",
				}),
			},
		}, {
			Description: "communities as a string, default-community",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"default-community": "nothing",
					"communities":       "private",
				}
			},
			Error: true,
		}, {
			Description: "communities, default-community empty",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"default-community": "",
					"communities": gin.H{
						"203.0.113.0/25":   "public",
						"203.0.113.128/25": "private",
					},
				}
			},
			Expected: Configuration{
				Communities: helpers.MustNewSubnetMap(map[string]string{
					"::/0":                     "public",
					"::ffff:203.0.113.0/121":   "public",
					"::ffff:203.0.113.128/121": "private",
				}),
			},
		}, {
			Description: "SNMP security parameters",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"security-parameters": gin.H{
						"user-name":                 "alfred",
						"authentication-protocol":   "sha",
						"authentication-passphrase": "hello",
						"privacy-protocol":          "aes",
						"privacy-passphrase":        "bye",
					},
				}
			},
			Expected: Configuration{
				Communities: helpers.MustNewSubnetMap(map[string]string{
					"::/0": "public",
				}),
				SecurityParameters: helpers.MustNewSubnetMap(map[string]SecurityParameters{
					"::/0": {
						UserName:                 "alfred",
						AuthenticationProtocol:   AuthProtocol(gosnmp.SHA),
						AuthenticationPassphrase: "hello",
						PrivacyProtocol:          PrivProtocol(gosnmp.AES),
						PrivacyPassphrase:        "bye",
					},
				}),
			},
		},
	})
}
