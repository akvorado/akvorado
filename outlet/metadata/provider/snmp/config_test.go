// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"testing"
	"time"

	"akvorado/common/helpers"
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
			Initial:        func() any { return Configuration{} },
			Configuration:  func() any { return nil },
			Expected:       Configuration{},
			SkipValidation: true,
		}, {
			Description:   "empty",
			Initial:       func() any { return Configuration{} },
			Configuration: func() any { return helpers.M{} },
			Expected: Configuration{
				Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
					"::/0": {Communities: []string{"public"}},
				}),
			},
			SkipValidation: true,
		}, {
			Description: "single port",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
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
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout": "200ms",
					"ports": helpers.M{
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
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
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
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout": "200ms",
					"communities": helpers.M{
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
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout":    "200ms",
					"default-community": "private",
					"communities": helpers.M{
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
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
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
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout":    "200ms",
					"default-community": "",
					"communities": helpers.M{
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
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout": "200ms",
					"security-parameters": helpers.M{
						"user-name":                 "alfred",
						"authentication-protocol":   "sha",
						"authentication-passphrase": "hello",
						"privacy-protocol":          "AES",
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
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout": "200ms",
					"security-parameters": helpers.M{
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
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout": "200ms",
					"security-parameters": helpers.M{
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
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout": "200ms",
					"security-parameters": helpers.M{
						"user-name":          "alfred",
						"privacy-protocol":   "aes192",
						"privacy-passphrase": "hello",
					},
				}
			},
			Error: true,
		}, {
			Description: "SNMP security parameters without authentication passphrase",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout": "200ms",
					"security-parameters": helpers.M{
						"user-name":               "alfred",
						"authentication-protocol": "sha",
					},
				}
			},
			Error: true,
		}, {
			Description: "SNMP security parameters without username",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout": "200ms",
					"credentials": helpers.M{
						"::/0": helpers.M{
							"authentication-protocol":   "sha",
							"authentication-passphrase": "hello",
						},
					},
				}
			},
			Error: true,
		}, {
			Description: "merge communities and security-parameters",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout": "200ms",
					"communities": helpers.M{
						"203.0.113.0/25":   "public",
						"203.0.113.128/25": "private",
					},
					"security-parameters": helpers.M{
						"::/0": helpers.M{
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
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout": "200ms",
					"communities": helpers.M{
						"203.0.113.0/25":   "public",
						"203.0.113.128/25": "private",
					},
					"security-parameters": helpers.M{
						"::/0": helpers.M{
							"user-name":                 "alfred",
							"authentication-protocol":   "sha",
							"authentication-passphrase": "hello",
						},
					},
					"credentials": helpers.M{
						"203.0.113.0/29": helpers.M{
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
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout": "200ms",
					"communities": helpers.M{
						"203.0.113.0/25":   "public",
						"203.0.113.128/25": "private",
					},
					"security-parameters": helpers.M{
						"203.0.113.2": helpers.M{
							"user-name":                 "alfred",
							"authentication-protocol":   "sha",
							"authentication-passphrase": "hello",
						},
					},
					"credentials": helpers.M{
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
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"poller-timeout": "200ms",
					"credentials": helpers.M{
						"203.0.113.0/25": helpers.M{
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
