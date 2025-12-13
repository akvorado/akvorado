// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package gnmi

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

func TestAuthenticationParameterMigration(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description: "minimal config",
			Initial:     func() any { return AuthenticationParameter{} },
			Configuration: func() any {
				return helpers.M{
					"username": "admin",
					"password": "secret",
				}
			},
			Expected: AuthenticationParameter{
				Username: "admin",
				Password: "secret",
				TLS: helpers.TLSConfiguration{
					Enable: true,
				},
			},
		}, {
			Description: "old insecure=true migrates to TLS.Enable=false",
			Initial:     func() any { return AuthenticationParameter{} },
			Configuration: func() any {
				return helpers.M{
					"username": "admin",
					"password": "secret",
					"insecure": true,
				}
			},
			Expected: AuthenticationParameter{
				Username: "admin",
				Password: "secret",
				TLS: helpers.TLSConfiguration{
					Enable: false,
				},
			},
		}, {
			Description: "old insecure=false migrates to TLS.Enable=true",
			Initial:     func() any { return AuthenticationParameter{} },
			Configuration: func() any {
				return helpers.M{
					"username": "admin",
					"password": "secret",
					"insecure": false,
				}
			},
			Expected: AuthenticationParameter{
				Username: "admin",
				Password: "secret",
				TLS: helpers.TLSConfiguration{
					Enable: true,
				},
			},
		}, {
			Description: "old skip-verify migrates to TLS.SkipVerify",
			Initial:     func() any { return AuthenticationParameter{} },
			Configuration: func() any {
				return helpers.M{
					"username":    "admin",
					"password":    "secret",
					"skip-verify": true,
				}
			},
			Expected: AuthenticationParameter{
				Username: "admin",
				Password: "secret",
				TLS: helpers.TLSConfiguration{
					Enable:     true,
					SkipVerify: true,
				},
			},
		}, {
			Description: "old TLS certificate fields migrate to TLS config",
			Initial:     func() any { return AuthenticationParameter{} },
			Configuration: func() any {
				return helpers.M{
					"username": "admin",
					"password": "secret",
					"tls-ca":   "/path/to/ca.crt",
					"tls-cert": "/path/to/cert.crt",
					"tls-key":  "/path/to/key.pem",
				}
			},
			Expected: AuthenticationParameter{
				Username: "admin",
				Password: "secret",
				TLS: helpers.TLSConfiguration{
					Enable:   true,
					CAFile:   "/path/to/ca.crt",
					CertFile: "/path/to/cert.crt",
					KeyFile:  "/path/to/key.pem",
				},
			},
		}, {
			Description: "new TLS config with default enable=true",
			Initial:     func() any { return AuthenticationParameter{} },
			Configuration: func() any {
				return helpers.M{
					"username": "admin",
					"password": "secret",
					"tls": helpers.M{
						"skip-verify": true,
					},
				}
			},
			Expected: AuthenticationParameter{
				Username: "admin",
				Password: "secret",
				TLS: helpers.TLSConfiguration{
					Enable:     true,
					SkipVerify: true,
				},
			},
		}, {
			Description: "new TLS config with explicit enable=false",
			Initial:     func() any { return AuthenticationParameter{} },
			Configuration: func() any {
				return helpers.M{
					"username": "admin",
					"password": "secret",
					"tls": helpers.M{
						"enable": false,
					},
				}
			},
			Expected: AuthenticationParameter{
				Username: "admin",
				Password: "secret",
				TLS: helpers.TLSConfiguration{
					Enable: false,
				},
			},
		}, {
			Description: "mixing old and new TLS config causes error",
			Initial:     func() any { return AuthenticationParameter{} },
			Configuration: func() any {
				return helpers.M{
					"username": "admin",
					"password": "secret",
					"insecure": true,
					"tls": helpers.M{
						"enable": false,
					},
				}
			},
			Error: true,
		},
	})
}

func TestDefaults(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description:    "nil",
			Initial:        func() any { return Configuration{} },
			Configuration:  func() any { return nil },
			Expected:       Configuration{},
			SkipValidation: true,
		}, {
			Description:    "empty",
			Initial:        func() any { return Configuration{} },
			Configuration:  func() any { return helpers.M{} },
			Expected:       Configuration{},
			SkipValidation: true,
		}, {
			Description: "override models",
			Initial: func() any {
				return Configuration{Timeout: time.Second, MinimalRefreshInterval: time.Minute}
			},
			Configuration: func() any {
				return helpers.M{
					"models": []helpers.M{
						{
							"name":                 "custom",
							"if-index-paths":       "/some/path",
							"if-description-paths": "/some/other/path",
							"if-name-paths":        "/something",
							"if-speed-paths": []helpers.M{
								{"path": "/path1", "unit": "mbps"},
								{"path": "/path2", "unit": "ethernet"},
							},
							"system-name-paths": "/another/path",
						},
					},
				}
			},
			Expected: Configuration{
				Timeout:                time.Second,
				MinimalRefreshInterval: time.Minute,
				Models: []Model{
					{
						Name:               "custom",
						IfIndexPaths:       []string{"/some/path"},
						IfDescriptionPaths: []string{"/some/other/path"},
						IfNamePaths:        []string{"/something"},
						IfSpeedPaths: []IfSpeedPath{
							{"/path1", SpeedMbps},
							{"/path2", SpeedEthernet},
						},
						SystemNamePaths: []string{"/another/path"},
					},
				},
			},
		}, {
			Description: "defaults only",
			Initial: func() any {
				return Configuration{Timeout: time.Second, MinimalRefreshInterval: time.Minute}
			},
			Configuration: func() any {
				return helpers.M{
					"models": []string{"defaults"},
				}
			},
			Expected: Configuration{
				Timeout:                time.Second,
				MinimalRefreshInterval: time.Minute,
				Models:                 DefaultModels(),
			},
		}, {
			Description: "defaults first",
			Initial: func() any {
				return Configuration{Timeout: time.Second, MinimalRefreshInterval: time.Minute}
			},
			Configuration: func() any {
				return helpers.M{
					"models": []any{
						"defaults",
						helpers.M{
							"name":                 "custom",
							"if-index-paths":       "/some/path",
							"if-description-paths": "/some/other/path",
							"if-name-paths":        "/something",
							"if-speed-paths": []helpers.M{
								{"path": "/path1", "unit": "mbps"},
								{"path": "/path2", "unit": "ethernet"},
							},
							"system-name-paths": "/another/path",
						},
					},
				}
			},
			Expected: Configuration{
				Timeout:                time.Second,
				MinimalRefreshInterval: time.Minute,
				Models: append(DefaultModels(), Model{
					Name:               "custom",
					IfIndexPaths:       []string{"/some/path"},
					IfDescriptionPaths: []string{"/some/other/path"},
					IfNamePaths:        []string{"/something"},
					IfSpeedPaths: []IfSpeedPath{
						{"/path1", SpeedMbps},
						{"/path2", SpeedEthernet},
					},
					SystemNamePaths: []string{"/another/path"},
				}),
			},
		}, {
			Description: "defaults last",
			Initial: func() any {
				return Configuration{Timeout: time.Second, MinimalRefreshInterval: time.Minute}
			},
			Configuration: func() any {
				return helpers.M{
					"models": []any{
						helpers.M{
							"name":                 "custom",
							"if-index-paths":       "/some/path",
							"if-description-paths": "/some/other/path",
							"if-name-paths":        "/something",
							"if-speed-paths": []helpers.M{
								{"path": "/path1", "unit": "mbps"},
								{"path": "/path2", "unit": "ethernet"},
							},
							"system-name-paths": "/another/path",
						},
						"defaults",
					},
				}
			},
			Expected: Configuration{
				Timeout:                time.Second,
				MinimalRefreshInterval: time.Minute,
				Models: append([]Model{
					{
						Name:               "custom",
						IfIndexPaths:       []string{"/some/path"},
						IfDescriptionPaths: []string{"/some/other/path"},
						IfNamePaths:        []string{"/something"},
						IfSpeedPaths: []IfSpeedPath{
							{"/path1", SpeedMbps},
							{"/path2", SpeedEthernet},
						},
						SystemNamePaths: []string{"/another/path"},
					},
				}, DefaultModels()...),
			},
		}, {
			Description: "defaults in the middle",
			Initial: func() any {
				return Configuration{Timeout: time.Second, MinimalRefreshInterval: time.Minute}
			},
			Configuration: func() any {
				return helpers.M{
					"models": []any{
						helpers.M{
							"name":                 "custom1",
							"if-index-paths":       "/some/path",
							"if-description-paths": "/some/other/path",
							"if-name-paths":        "/something",
							"if-speed-paths": []helpers.M{
								{"path": "/path1", "unit": "mbps"},
								{"path": "/path2", "unit": "ethernet"},
							},
							"system-name-paths": "/another/path",
						},
						"defaults",
						helpers.M{
							"name":                 "custom2",
							"if-index-paths":       "/some/path",
							"if-description-paths": "/some/other/path",
							"if-name-paths":        "/something",
							"if-speed-paths": []helpers.M{
								{"path": "/path1", "unit": "mbps"},
								{"path": "/path2", "unit": "ethernet"},
							},
							"system-name-paths": "/another/path",
						},
					},
				}
			},
			Expected: Configuration{
				Timeout:                time.Second,
				MinimalRefreshInterval: time.Minute,
				Models: append([]Model{
					{
						Name:               "custom1",
						IfIndexPaths:       []string{"/some/path"},
						IfDescriptionPaths: []string{"/some/other/path"},
						IfNamePaths:        []string{"/something"},
						IfSpeedPaths: []IfSpeedPath{
							{"/path1", SpeedMbps},
							{"/path2", SpeedEthernet},
						},
						SystemNamePaths: []string{"/another/path"},
					},
				}, append(DefaultModels(), Model{
					Name:               "custom2",
					IfIndexPaths:       []string{"/some/path"},
					IfDescriptionPaths: []string{"/some/other/path"},
					IfNamePaths:        []string{"/something"},
					IfSpeedPaths: []IfSpeedPath{
						{"/path1", SpeedMbps},
						{"/path2", SpeedEthernet},
					},
					SystemNamePaths: []string{"/another/path"},
				})...),
			},
		},
	})
}
