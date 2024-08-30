package schema

import (
	"testing"

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
			Description:    "empty",
			Initial:        func() interface{} { return Configuration{} },
			Configuration:  func() interface{} { return gin.H{} },
			Expected:       Configuration{},
			SkipValidation: true,
		}, {
			Description: "empty-custom-dict",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"custom-dictionaries": gin.H{},
				}
			},
			Expected: Configuration{
				CustomDictionaries: map[string]CustomDict{},
			},
			SkipValidation: true,
		}, {
			Description: "custom-dict-file",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"custom-dictionaries": gin.H{
						"test": gin.H{
							"keys":        []gin.H{{"name": "testkey"}},
							"attributes":  []gin.H{{"name": "testattribute"}},
							"source-type": "file",
						},
					},
				}
			},
			Expected: Configuration{
				CustomDictionaries: map[string]CustomDict{
					"test": {
						Keys: []CustomDictKey{
							{
								Name: "testkey",
								Type: "String",
							},
						},
						Attributes: []CustomDictAttribute{
							{
								Name: "testattribute",
								Type: "String",
							},
						},
						Layout:     "hashed",
						SourceType: SourceFile,
					},
				},
			},
			SkipValidation: true,
		}, {
			Description: "custom-dict-s3",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"custom-dictionaries": gin.H{
						"test": gin.H{
							"source-type": "s3",
						},
					},
				}
			},
			Expected: Configuration{
				CustomDictionaries: map[string]CustomDict{
					"test": {
						Layout:     "hashed",
						SourceType: SourceS3,
					},
				},
			},
			SkipValidation: true,
		}, {
			Description: "custom-dict-http",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"custom-dictionaries": gin.H{
						"test": gin.H{
							"source-type": "http",
						},
					},
				}
			},
			Expected: Configuration{
				CustomDictionaries: map[string]CustomDict{
					"test": {
						Layout:     "hashed",
						SourceType: SourceHTTP,
					},
				},
			},
			SkipValidation: true,
		}, {
			Description: "custom-dict-invalid-src-type",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"custom-dictionaries": gin.H{
						"test": gin.H{
							"source-type": "invalid",
						},
					},
				}
			},
			Expected:       Configuration{},
			Error:          true,
			SkipValidation: true,
		},
	})
}
