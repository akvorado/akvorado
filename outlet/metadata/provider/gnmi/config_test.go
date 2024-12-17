// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package gnmi

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
)

func TestDefaultConfiguration(t *testing.T) {
	if err := helpers.Validate.Struct(DefaultConfiguration()); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}

func TestDefaults(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description:    "nil",
			Initial:        func() interface{} { return Configuration{} },
			Configuration:  func() interface{} { return nil },
			Expected:       Configuration{},
			SkipValidation: true,
		}, {
			Description:    "empty",
			Initial:        func() interface{} { return Configuration{} },
			Configuration:  func() interface{} { return gin.H{} },
			Expected:       Configuration{},
			SkipValidation: true,
		}, {
			Description: "override models",
			Initial: func() interface{} {
				return Configuration{Timeout: time.Second, MinimalRefreshInterval: time.Minute}
			},
			Configuration: func() interface{} {
				return gin.H{
					"models": []gin.H{
						{
							"name":                 "custom",
							"if-index-paths":       "/some/path",
							"if-description-paths": "/some/other/path",
							"if-name-paths":        "/something",
							"if-speed-paths": []gin.H{
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
			Initial: func() interface{} {
				return Configuration{Timeout: time.Second, MinimalRefreshInterval: time.Minute}
			},
			Configuration: func() interface{} {
				return gin.H{
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
			Initial: func() interface{} {
				return Configuration{Timeout: time.Second, MinimalRefreshInterval: time.Minute}
			},
			Configuration: func() interface{} {
				return gin.H{
					"models": []interface{}{
						"defaults",
						gin.H{
							"name":                 "custom",
							"if-index-paths":       "/some/path",
							"if-description-paths": "/some/other/path",
							"if-name-paths":        "/something",
							"if-speed-paths": []gin.H{
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
			Initial: func() interface{} {
				return Configuration{Timeout: time.Second, MinimalRefreshInterval: time.Minute}
			},
			Configuration: func() interface{} {
				return gin.H{
					"models": []interface{}{
						gin.H{
							"name":                 "custom",
							"if-index-paths":       "/some/path",
							"if-description-paths": "/some/other/path",
							"if-name-paths":        "/something",
							"if-speed-paths": []gin.H{
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
			Initial: func() interface{} {
				return Configuration{Timeout: time.Second, MinimalRefreshInterval: time.Minute}
			},
			Configuration: func() interface{} {
				return gin.H{
					"models": []interface{}{
						gin.H{
							"name":                 "custom1",
							"if-index-paths":       "/some/path",
							"if-description-paths": "/some/other/path",
							"if-name-paths":        "/something",
							"if-speed-paths": []gin.H{
								{"path": "/path1", "unit": "mbps"},
								{"path": "/path2", "unit": "ethernet"},
							},
							"system-name-paths": "/another/path",
						},
						"defaults",
						gin.H{
							"name":                 "custom2",
							"if-index-paths":       "/some/path",
							"if-description-paths": "/some/other/path",
							"if-name-paths":        "/something",
							"if-speed-paths": []gin.H{
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
