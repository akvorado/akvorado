// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
)

func TestMapStructureMatchName(t *testing.T) {
	cases := []struct {
		mapKey    string
		fieldName string
		expected  bool
	}{
		{"one", "one", true},
		{"one", "One", true},
		{"one-two", "OneTwo", true},
		{"onetwo", "OneTwo", true},
		{"One-Two", "OneTwo", true},
		{"two", "one", false},
	}
	for _, tc := range cases {
		got := MapStructureMatchName(tc.mapKey, tc.fieldName)
		if got && !tc.expected {
			t.Errorf("%q == %q but expected !=", tc.mapKey, tc.fieldName)
		} else if !got && tc.expected {
			t.Errorf("%q != %q but expected ==", tc.mapKey, tc.fieldName)
		}
	}
}

func TestProtectedDecodeHook(t *testing.T) {
	var configuration struct {
		A string
		B string
	}
	panicHook := func(from, _ reflect.Type, data interface{}) (interface{}, error) {
		if from.Kind() == reflect.String {
			panic(errors.New("noooo"))
		}
		return data, nil
	}
	decoder, err := mapstructure.NewDecoder(GetMapStructureDecoderConfig(&configuration, panicHook))
	if err != nil {
		t.Fatalf("NewDecoder() error:\n%+v", err)
	}
	err = decoder.Decode(gin.H{"A": "hello", "B": "bye"})
	if err == nil {
		t.Fatal("Decode() did not error")
	} else {
		got := strings.Split(err.Error(), "\n")
		expected := []string{
			`2 error(s) decoding:`,
			``,
			`* error decoding 'A': internal error while parsing: noooo`,
			`* error decoding 'B': internal error while parsing: noooo`,
		}
		if diff := Diff(got, expected); diff != "" {
			t.Fatalf("Decode() error:\n%s", diff)
		}
	}
}

func TestDefaultValuesConfig(t *testing.T) {
	type InnerConfiguration struct {
		AA string
		BB string
		CC int
	}
	type OuterConfiguration struct {
		DD []InnerConfiguration
	}
	RegisterMapstructureUnmarshallerHook(DefaultValuesUnmarshallerHook(InnerConfiguration{
		BB: "hello",
		CC: 10,
	}))
	TestConfigurationDecode(t, ConfigurationDecodeCases{
		{
			Initial: func() interface{} { return OuterConfiguration{} },
			Configuration: func() interface{} {
				return gin.H{
					"dd": []gin.H{
						{
							"aa": "hello1",
							"bb": "hello2",
							"cc": 43,
						},
						{"cc": 44},
						{"aa": "bye"},
					},
				}
			},
			Expected: OuterConfiguration{
				DD: []InnerConfiguration{
					{
						AA: "hello1",
						BB: "hello2",
						CC: 43,
					}, {
						AA: "",
						BB: "hello",
						CC: 44,
					}, {
						AA: "bye",
						BB: "hello",
						CC: 10,
					},
				},
			},
		},
	})
}

func TestParametrizedConfig(t *testing.T) {
	type InnerConfigurationType1 struct {
		CC string
		DD string
	}
	type InnerConfigurationType2 struct {
		CC string
		EE string
	}
	type OuterConfiguration struct {
		AA     string
		BB     string
		Config interface{}
	}
	available := map[string](func() interface{}){
		"type1": func() interface{} {
			return InnerConfigurationType1{
				CC: "cc1",
				DD: "dd1",
			}
		},
		"type2": func() interface{} {
			return InnerConfigurationType2{
				CC: "cc2",
				EE: "ee2",
			}
		},
	}
	RegisterMapstructureUnmarshallerHook(ParametrizedConfigurationUnmarshallerHook(OuterConfiguration{}, available))

	t.Run("unmarshal", func(t *testing.T) {
		TestConfigurationDecode(t, ConfigurationDecodeCases{
			{
				Description: "type1",
				Initial:     func() interface{} { return OuterConfiguration{} },
				Configuration: func() interface{} {
					return gin.H{
						"type": "type1",
						"aa":   "a1",
						"bb":   "b1",
						"cc":   "c1",
						"dd":   "d1",
					}
				},
				Expected: OuterConfiguration{
					AA: "a1",
					BB: "b1",
					Config: InnerConfigurationType1{
						CC: "c1",
						DD: "d1",
					},
				},
			}, {
				Description: "type2",
				Initial:     func() interface{} { return OuterConfiguration{} },
				Configuration: func() interface{} {
					return gin.H{
						"type": "type2",
						"aa":   "a2",
						"bb":   "b2",
						"cc":   "c2",
						"ee":   "e2",
					}
				},
				Expected: OuterConfiguration{
					AA: "a2",
					BB: "b2",
					Config: InnerConfigurationType2{
						CC: "c2",
						EE: "e2",
					},
				},
			}, {
				Description: "unknown type",
				Initial:     func() interface{} { return OuterConfiguration{} },
				Configuration: func() interface{} {
					return gin.H{
						"type": "type3",
						"aa":   "a2",
						"bb":   "b2",
						"cc":   "c2",
						"ee":   "e2",
					}
				},
				Error: true,
			},
		})
	})
	t.Run("marshal", func(t *testing.T) {
		config1 := OuterConfiguration{
			AA: "a1",
			BB: "b1",
			Config: InnerConfigurationType1{
				CC: "c1",
				DD: "d1",
			},
		}
		expected1 := gin.H{
			"type": "type1",
			"aa":   "a1",
			"bb":   "b1",
			"cc":   "c1",
			"dd":   "d1",
		}
		got1, err := ParametrizedConfigurationMarshalYAML(config1, available)
		if err != nil {
			t.Fatalf("ParametrizedConfigurationMarshalYAML() error:\n%+v", err)
		}
		if diff := Diff(got1, expected1); diff != "" {
			t.Fatalf("ParametrizedConfigurationMarshalYAML() (-got, +want):\n%s", diff)
		}

		config2 := OuterConfiguration{
			AA: "a2",
			BB: "b2",
			Config: InnerConfigurationType2{
				CC: "c2",
				EE: "e2",
			},
		}
		expected2 := gin.H{
			"type": "type2",
			"aa":   "a2",
			"bb":   "b2",
			"cc":   "c2",
			"ee":   "e2",
		}
		got2, err := ParametrizedConfigurationMarshalYAML(config2, available)
		if err != nil {
			t.Fatalf("ParametrizedConfigurationMarshalYAML() error:\n%+v", err)
		}
		if diff := Diff(got2, expected2); diff != "" {
			t.Fatalf("ParametrizedConfigurationMarshalYAML() (-got, +want):\n%s", diff)
		}

		config3 := OuterConfiguration{
			AA: "a3",
			BB: "b3",
			Config: struct {
				FF string
			}{},
		}
		if _, err := ParametrizedConfigurationMarshalYAML(config3, available); err == nil {
			t.Fatal("ParametrizedConfigurationMarshalYAML() did not error")
		}
	})
}
