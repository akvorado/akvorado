package flow

import (
	"akvorado/flow/input/file"
	"akvorado/flow/input/udp"
	"akvorado/helpers"
	"strings"
	"testing"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"
)

func TestDecodeConfiguration(t *testing.T) {
	cases := []struct {
		Name     string
		From     interface{}
		Source   interface{}
		Expected interface{}
	}{
		{
			Name: "from empty configuration",
			From: Configuration{},
			Source: map[string]interface{}{
				"workers": 10,
				"inputs": []map[string]interface{}{
					map[string]interface{}{
						"type":    "udp",
						"decoder": "netflow",
						"listen":  "192.0.2.1:2055",
						"workers": 3,
					},
				},
			},
			Expected: Configuration{
				Workers: 10,
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config: &udp.Configuration{
						Workers:   3,
						QueueSize: 100000,
						Listen:    "192.0.2.1:2055",
					},
				}},
			},
		}, {
			Name: "from existing configuration",
			From: Configuration{
				Workers: 10,
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config:  &udp.DefaultConfiguration,
				}},
			},
			Source: map[string]interface{}{
				"workers": 10,
				"inputs": []map[string]interface{}{
					map[string]interface{}{
						"type":    "udp",
						"decoder": "netflow",
						"listen":  "192.0.2.1:2055",
						"workers": 3,
					},
				},
			},
			Expected: Configuration{
				Workers: 10,
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config: &udp.Configuration{
						Workers:   3,
						QueueSize: 100000,
						Listen:    "192.0.2.1:2055",
					},
				}},
			},
		}, {
			Name: "change type",
			From: Configuration{
				Workers: 10,
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config:  &udp.DefaultConfiguration,
				}},
			},
			Source: map[string]interface{}{
				"workers": 10,
				"inputs": []map[string]interface{}{
					map[string]interface{}{
						"type":  "file",
						"paths": []string{"file1", "file2"},
					},
				},
			},
			Expected: Configuration{
				Workers: 10,
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config: &file.Configuration{
						Paths: []string{"file1", "file2"},
					},
				}},
			},
		}, {
			Name: "only set one item",
			From: Configuration{
				Workers: 10,
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config:  &udp.DefaultConfiguration,
				}},
			},
			Source: map[string]interface{}{
				"inputs": []map[string]interface{}{
					map[string]interface{}{
						"listen": "192.0.2.1:2055",
					},
				},
			},
			Expected: Configuration{
				Workers: 10,
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config: &udp.Configuration{
						Workers:   1,
						QueueSize: 100000,
						Listen:    "192.0.2.1:2055",
					},
				}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := tc.From

			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				Result:           &got,
				ErrorUnused:      true,
				Metadata:         nil,
				WeaklyTypedInput: true,
				MatchName: func(mapKey, fieldName string) bool {
					key := strings.ToLower(strings.ReplaceAll(mapKey, "-", ""))
					field := strings.ToLower(fieldName)
					return key == field
				},
				DecodeHook: ConfigurationUnmarshalerHook(),
			})
			if err != nil {
				t.Fatalf("NewDecoder() error:\n%+v", err)
			}
			if err := decoder.Decode(tc.Source); err != nil {
				t.Fatalf("Decode() error:\n%+v", err)
			}

			expected := tc.Expected
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("Decode() (-got, +want):\n%s", diff)
			}
		})
	}

	// Check we didn't alter the default value
	if diff := helpers.Diff(udp.DefaultConfiguration, udp.Configuration{
		Workers:   1,
		QueueSize: 100000,
	}); diff != "" {
		t.Fatalf("Default configuration altered (-got, +want):\n%s", diff)
	}
}

func TestMarshalYAML(t *testing.T) {
	cfg := Configuration{
		Workers: 10,
		Inputs: []InputConfiguration{
			{
				Decoder: "netflow",
				Config: &udp.Configuration{
					Listen:    "192.0.2.11:2055",
					QueueSize: 1000,
					Workers:   3,
				},
			},
		},
	}
	got, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error:\n%+v", err)
	}
	expected := `inputs:
- decoder: netflow
  listen: 192.0.2.11:2055
  queuesize: 1000
  type: udp
  workers: 3
workers: 10
`
	if diff := helpers.Diff(strings.Split(string(got), "\n"), strings.Split(expected, "\n")); diff != "" {
		t.Fatalf("Marshal() (-got, +want):\n%s", diff)
	}
}
