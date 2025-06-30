// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"testing"
	"time"

	"akvorado/common/helpers"

	"github.com/gin-gonic/gin"
)

func TestDefaultConfiguration(t *testing.T) {
	if err := helpers.Validate.Struct(DefaultConfiguration()); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}

func TestConfigurationUnmarshallerHook(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description:    "nil",
			Initial:        func() interface{} { return Configuration{} },
			Configuration:  func() interface{} { return nil },
			Expected:       Configuration{},
			SkipValidation: true,
		}, {
			Description:   "empty",
			Initial:       func() interface{} { return Configuration{} },
			Configuration: func() interface{} { return gin.H{} },
			Expected: Configuration{
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {Communities: []string{"public"}},
				}),
			},
			SkipValidation: true,
		}, {
			Description: "single port",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
					"ports":          "1161",
				}
			},
			Expected: Configuration{
				PollerTimeout: 200 * time.Millisecond,
				Ports: helpers.MustNewSubnetMap(map[string]uint16{
					"::/0": 1161,
				}),
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {Communities: []string{"public"}},
				}),
			},
		}, {
			Description: "per-prefix port",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
					"ports": gin.H{
						"2001:db8:1::/48": 1161,
						"2001:db8:2::/48": 1162,
					},
				}
			},
			Expected: Configuration{
				PollerTimeout: 200 * time.Millisecond,
				Ports: helpers.MustNewSubnetMap(map[string]uint16{
					"2001:db8:1::/48": 1161,
					"2001:db8:2::/48": 1162,
				}),
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {Communities: []string{"public"}},
				}),
			},
		}, {
			Description: "no communities, no default community",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-retries": 10,
					"poller-timeout": "200ms",
				}
			},
			Expected: Configuration{
				PollerRetries: 10,
				PollerTimeout: 200 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {Communities: []string{"public"}},
				}),
			},
		}, {
			Description: "communities, no default community",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
					"communities": gin.H{
						"203.0.113.0/25":   "public",
						"203.0.113.128/25": "private",
					},
				}
			},
			Expected: Configuration{
				PollerTimeout: 200 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0":                     {Communities: []string{"public"}},
					"::ffff:203.0.113.0/121":   {Communities: []string{"public"}},
					"::ffff:203.0.113.128/121": {Communities: []string{"private"}},
				}),
			},
		}, {
			Description: "communities, default community",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout":    "200ms",
					"default-community": "private",
					"communities": gin.H{
						"203.0.113.0/25":   "public",
						"203.0.113.128/25": "private",
					},
				}
			},
			Expected: Configuration{
				PollerTimeout: 200 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0":                     {Communities: []string{"private"}},
					"::ffff:203.0.113.0/121":   {Communities: []string{"public"}},
					"::ffff:203.0.113.128/121": {Communities: []string{"private"}},
				}),
			},
		}, {
			Description: "communities as a string",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
					"communities":    "private",
				}
			},
			Expected: Configuration{
				PollerTimeout: 200 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {Communities: []string{"private"}},
				}),
			},
		}, {
			Description: "communities, default-community empty",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout":    "200ms",
					"default-community": "",
					"communities": gin.H{
						"203.0.113.0/25":   "public",
						"203.0.113.128/25": "private",
					},
				}
			},
			Expected: Configuration{
				PollerTimeout: 200 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0":                     {Communities: []string{"public"}},
					"::ffff:203.0.113.0/121":   {Communities: []string{"public"}},
					"::ffff:203.0.113.128/121": {Communities: []string{"private"}},
				}),
			},
		}, {
			Description: "SNMP security parameters",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
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
				PollerTimeout: 200 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {
						UserName:                 "alfred",
						AuthenticationProtocol:   AuthProtocolSHA,
						AuthenticationPassphrase: "hello",
						PrivacyProtocol:          PrivProtocolAES,
						PrivacyPassphrase:        "bye",
					},
				}),
			},
		}, {
			Description: "SNMP security parameters with AES256C",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
					"security-parameters": gin.H{
						"user-name":                 "alfred",
						"authentication-protocol":   "sha",
						"authentication-passphrase": "hello",
						"privacy-protocol":          "aes256-c",
						"privacy-passphrase":        "bye",
					},
				}
			},
			Expected: Configuration{
				PollerTimeout: 200 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {
						UserName:                 "alfred",
						AuthenticationProtocol:   AuthProtocolSHA,
						AuthenticationPassphrase: "hello",
						PrivacyProtocol:          PrivProtocolAES256C,
						PrivacyPassphrase:        "bye",
					},
				}),
			},
		}, {
			Description: "SNMP security parameters without privacy protocol",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
					"security-parameters": gin.H{
						"user-name":                 "alfred",
						"authentication-protocol":   "sha",
						"authentication-passphrase": "hello",
					},
				}
			},
			Expected: Configuration{
				PollerTimeout: 200 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {
						UserName:                 "alfred",
						AuthenticationProtocol:   AuthProtocolSHA,
						AuthenticationPassphrase: "hello",
					},
				}),
			},
		}, {
			Description: "SNMP security parameters without authentication protocol",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
					"security-parameters": gin.H{
						"user-name":          "alfred",
						"privacy-protocol":   "aes192",
						"privacy-passphrase": "hello",
					},
				}
			},
			Error: true,
		}, {
			Description: "SNMP security parameters without authentication passphrase",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
					"security-parameters": gin.H{
						"user-name":               "alfred",
						"authentication-protocol": "sha",
					},
				}
			},
			Error: true,
		}, {
			Description: "SNMP security parameters without username",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
					"credentials": gin.H{
						"::/0": gin.H{
							"authentication-protocol":   "sha",
							"authentication-passphrase": "hello",
						},
					},
				}
			},
			Error: true,
		}, {
			Description: "merge communities and security-parameters",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
					"communities": gin.H{
						"203.0.113.0/25":   "public",
						"203.0.113.128/25": "private",
					},
					"security-parameters": gin.H{
						"::/0": gin.H{
							"user-name":                 "alfred",
							"authentication-protocol":   "sha",
							"authentication-passphrase": "hello",
						},
					},
				}
			},
			Expected: Configuration{
				PollerTimeout: 200 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {
						UserName:                 "alfred",
						AuthenticationProtocol:   AuthProtocolSHA,
						AuthenticationPassphrase: "hello",
					},
					"::ffff:203.0.113.0/121":   {Communities: []string{"public"}},
					"::ffff:203.0.113.128/121": {Communities: []string{"private"}},
				}),
			},
		}, {
			Description: "merge communities, security-parameters and credentials",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
					"communities": gin.H{
						"203.0.113.0/25":   "public",
						"203.0.113.128/25": "private",
					},
					"security-parameters": gin.H{
						"::/0": gin.H{
							"user-name":                 "alfred",
							"authentication-protocol":   "sha",
							"authentication-passphrase": "hello",
						},
					},
					"credentials": gin.H{
						"203.0.113.0/29": gin.H{
							"communities": "something",
						},
					},
				}
			},
			Expected: Configuration{
				PollerTimeout: 200 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {
						UserName:                 "alfred",
						AuthenticationProtocol:   AuthProtocolSHA,
						AuthenticationPassphrase: "hello",
					},
					"::ffff:203.0.113.0/125":   {Communities: []string{"something"}},
					"::ffff:203.0.113.0/121":   {Communities: []string{"public"}},
					"::ffff:203.0.113.128/121": {Communities: []string{"private"}},
				}),
			},
		}, {
			Description: "merge communities, security-parameters and default credentials",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
					"communities": gin.H{
						"203.0.113.0/25":   "public",
						"203.0.113.128/25": "private",
					},
					"security-parameters": gin.H{
						"203.0.113.2": gin.H{
							"user-name":                 "alfred",
							"authentication-protocol":   "sha",
							"authentication-passphrase": "hello",
						},
					},
					"credentials": gin.H{
						"communities": "something",
					},
				}
			},
			Expected: Configuration{
				PollerTimeout: 200 * time.Millisecond,
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {Communities: []string{"something"}},
					"203.0.113.2": {
						UserName:                 "alfred",
						AuthenticationProtocol:   AuthProtocolSHA,
						AuthenticationPassphrase: "hello",
					},
					"::ffff:203.0.113.0/121":   {Communities: []string{"public"}},
					"::ffff:203.0.113.128/121": {Communities: []string{"private"}},
				}),
			},
		}, {
			Description: "conflicting SNMP version",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"poller-timeout": "200ms",
					"credentials": gin.H{
						"203.0.113.0/25": gin.H{
							"communities":               "private",
							"user-name":                 "alfred",
							"authentication-protocol":   "sha",
							"authentication-passphrase": "hello",
						},
					},
				}
			},
			Error: true,
		},
	})
}
