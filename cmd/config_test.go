// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v2"

	"akvorado/cmd"
	"akvorado/common/helpers"
)

type dummyConfiguration struct {
	Module1 dummyModule1Configuration
	Module2 dummyModule2Configuration
}
type dummyModule1Configuration struct {
	Listen  string `validate:"listen"`
	Topic   string `validate:"gte=3"`
	Workers int    `validate:"gte=1"`
}
type dummyModule2Configuration struct {
	Details     dummyModule2DetailsConfiguration
	Elements    []dummyModule2ElementsConfiguration
	MoreDetails `mapstructure:",squash" yaml:",inline"`
}
type MoreDetails struct {
	Stuff string
}
type dummyModule2ElementsConfiguration struct {
	Name  string
	Gauge int
}
type dummyModule2DetailsConfiguration struct {
	Workers       int
	IntervalValue time.Duration
}

func (c *dummyConfiguration) Reset() {
	*c = dummyConfiguration{
		Module1: dummyModule1Configuration{
			Listen:  "127.0.0.1:8080",
			Topic:   "nothingness",
			Workers: 100,
		},
		Module2: dummyModule2Configuration{
			MoreDetails: MoreDetails{
				Stuff: "hello",
			},
			Details: dummyModule2DetailsConfiguration{
				Workers:       1,
				IntervalValue: time.Minute,
			},
			Elements: []dummyModule2ElementsConfiguration{
				{
					Name:  "el1",
					Gauge: 10,
				}, {
					Name:  "el2",
					Gauge: 11,
				},
			},
		},
	}
}

func TestValidation(t *testing.T) {
	config := `---
module1:
 topic: fl
 workers: -5
`
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	ioutil.WriteFile(configFile, []byte(config), 0644)

	c := cmd.ConfigRelatedOptions{
		Path: configFile,
	}

	parsed := dummyConfiguration{}
	out := bytes.NewBuffer([]byte{})
	if err := c.Parse(out, "dummy", &parsed); err == nil {
		t.Fatal("Parse() didn't error")
	} else if diff := helpers.Diff(err.Error(), `invalid configuration:
Key: 'dummyConfiguration.Module1.Topic' Error:Field validation for 'Topic' failed on the 'gte' tag
Key: 'dummyConfiguration.Module1.Workers' Error:Field validation for 'Workers' failed on the 'gte' tag`); diff != "" {
		t.Fatalf("Parse() (-got, +want):\n%s", diff)
	}
}

func TestDump(t *testing.T) {
	// Configuration file
	config := `---
module1:
 topic: flows
module2:
 details:
  workers: 5
  interval-value: 20m
 stuff: bye
 elements:
  - name: first
    gauge: 67
  - name: second
`
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	ioutil.WriteFile(configFile, []byte(config), 0644)

	c := cmd.ConfigRelatedOptions{
		Path: configFile,
		Dump: true,
	}

	parsed := dummyConfiguration{}
	out := bytes.NewBuffer([]byte{})
	if err := c.Parse(out, "dummy", &parsed); err != nil {
		t.Fatalf("Parse() error:\n%+v", err)
	}
	// Expected configuration
	expected := dummyConfiguration{
		Module1: dummyModule1Configuration{
			Listen:  "127.0.0.1:8080",
			Topic:   "flows",
			Workers: 100,
		},
		Module2: dummyModule2Configuration{
			MoreDetails: MoreDetails{
				Stuff: "bye",
			},
			Details: dummyModule2DetailsConfiguration{
				Workers:       5,
				IntervalValue: 20 * time.Minute,
			},
			Elements: []dummyModule2ElementsConfiguration{
				{"first", 67},
				{"second", 0},
			},
		},
	}
	if diff := helpers.Diff(parsed, expected); diff != "" {
		t.Errorf("Parse() (-got, +want):\n%s", diff)
	}

	var gotRaw map[string]gin.H
	if err := yaml.Unmarshal(out.Bytes(), &gotRaw); err != nil {
		t.Fatalf("Unmarshal() error:\n%+v", err)
	}
	expectedRaw := gin.H{
		"module1": gin.H{
			"listen":  "127.0.0.1:8080",
			"topic":   "flows",
			"workers": 100,
		},
		"module2": gin.H{
			"stuff": "bye",
			"details": gin.H{
				"workers":       5,
				"intervalvalue": "20m0s",
			},
			"elements": []interface{}{
				gin.H{
					"name":  "first",
					"gauge": 67,
				},
				gin.H{
					"name":  "second",
					"gauge": 0,
				},
			},
		},
	}
	if diff := helpers.Diff(gotRaw, expectedRaw); diff != "" {
		t.Errorf("Parse() (-got, +want):\n%s", diff)
	}
}

func TestEnvOverride(t *testing.T) {
	// Configuration file
	config := `---
module1:
 topic: flows
module2:
 details:
  workers: 5
  interval-value: 20m
`
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	ioutil.WriteFile(configFile, []byte(config), 0644)

	// Environment
	clean := func() {
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, "AKVORADO_DUMMY_") {
				os.Unsetenv(strings.Split(env, "=")[0])
			}
		}
	}
	clean()
	defer clean()
	os.Setenv("AKVORADO_DUMMY_MODULE1_LISTEN", "127.0.0.1:9000")
	os.Setenv("AKVORADO_DUMMY_MODULE1_TOPIC", "something")
	os.Setenv("AKVORADO_DUMMY_MODULE2_DETAILS_INTERVALVALUE", "10m")
	os.Setenv("AKVORADO_DUMMY_MODULE2_STUFF", "bye")
	os.Setenv("AKVORADO_DUMMY_MODULE2_ELEMENTS_0_NAME", "something")
	os.Setenv("AKVORADO_DUMMY_MODULE2_ELEMENTS_0_GAUGE", "18")
	os.Setenv("AKVORADO_DUMMY_MODULE2_ELEMENTS_1_NAME", "something else")
	os.Setenv("AKVORADO_DUMMY_MODULE2_ELEMENTS_1_GAUGE", "7")

	c := cmd.ConfigRelatedOptions{
		Path: configFile,
		Dump: true,
	}

	parsed := dummyConfiguration{}
	out := bytes.NewBuffer([]byte{})
	if err := c.Parse(out, "dummy", &parsed); err != nil {
		t.Fatalf("Parse() error:\n%+v", err)
	}
	// Expected configuration
	expected := dummyConfiguration{
		Module1: dummyModule1Configuration{
			Listen:  "127.0.0.1:9000",
			Topic:   "something",
			Workers: 100,
		},
		Module2: dummyModule2Configuration{
			MoreDetails: MoreDetails{
				Stuff: "bye",
			},
			Details: dummyModule2DetailsConfiguration{
				Workers:       5,
				IntervalValue: 10 * time.Minute,
			},
			Elements: []dummyModule2ElementsConfiguration{
				{"something", 18},
				{"something else", 7},
			},
		},
	}
	if diff := helpers.Diff(parsed, expected); diff != "" {
		t.Errorf("Parse() (-got, +want):\n%s", diff)
	}
}

func TestHTTPConfiguration(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
		fmt.Fprint(w, `---
module1:
 topic: flows
module2:
 details:
  workers: 5
  interval-value: 20m
 stuff: bye
 elements:
   - {"name": "first", "gauge": 67}
   - {"name": "second"}
`)
	}))
	defer ts.Close()

	c := cmd.ConfigRelatedOptions{
		Path: ts.URL,
		Dump: true,
	}

	parsed := dummyConfiguration{}
	out := bytes.NewBuffer([]byte{})
	if err := c.Parse(out, "dummy", &parsed); err != nil {
		t.Fatalf("Parse() error:\n%+v", err)
	}
	// Expected configuration
	expected := dummyConfiguration{
		Module1: dummyModule1Configuration{
			Listen:  "127.0.0.1:8080",
			Topic:   "flows",
			Workers: 100,
		},
		Module2: dummyModule2Configuration{
			MoreDetails: MoreDetails{
				Stuff: "bye",
			},
			Details: dummyModule2DetailsConfiguration{
				Workers:       5,
				IntervalValue: 20 * time.Minute,
			},
			Elements: []dummyModule2ElementsConfiguration{
				{"first", 67},
				{"second", 0},
			},
		},
	}
	if diff := helpers.Diff(parsed, expected); diff != "" {
		t.Errorf("Parse() (-got, +want):\n%s", diff)
	}
}

func TestUnused(t *testing.T) {
	t.Run("ignored fields", func(t *testing.T) {
		config := `---
.unused: should be ignored
module1:
 .too: nope
 topic: flow
 workers: 10
`
		configFile := filepath.Join(t.TempDir(), "config.yaml")
		ioutil.WriteFile(configFile, []byte(config), 0644)

		c := cmd.ConfigRelatedOptions{Path: configFile}

		parsed := dummyConfiguration{}
		out := bytes.NewBuffer([]byte{})
		if err := c.Parse(out, "dummy", &parsed); err != nil {
			t.Fatalf("Parse() error:\n%+v", err)
		}
	})

	t.Run("unused fields", func(t *testing.T) {
		config := `---
unused: should not be ignored
module1:
 extra: 111
 topic: flow
 workers: 10
`
		configFile := filepath.Join(t.TempDir(), "config.yaml")
		ioutil.WriteFile(configFile, []byte(config), 0644)

		c := cmd.ConfigRelatedOptions{Path: configFile}

		parsed := dummyConfiguration{}
		out := bytes.NewBuffer([]byte{})
		if err := c.Parse(out, "dummy", &parsed); err == nil {
			t.Fatal("Parse() didn't error")
		} else if diff := helpers.Diff(err.Error(), `invalid configuration:
invalid key "Module1.extra"
invalid key "unused"`); diff != "" {
			t.Fatalf("Parse() (-got, +want):\n%s", diff)
		}
	})
}

func TestDefaultInSlice(t *testing.T) {
	try := func(t *testing.T, parse func(cmd.ConfigRelatedOptions, *bytes.Buffer) interface{}) {
		// Configuration file
		config := `---
modules:
- module1:
    topic: flows1
- module1:
    topic: flows2
`
		configFile := filepath.Join(t.TempDir(), "config.yaml")
		ioutil.WriteFile(configFile, []byte(config), 0644)

		c := cmd.ConfigRelatedOptions{
			Path: configFile,
			Dump: true,
		}

		out := bytes.NewBuffer([]byte{})
		parsed := parse(c, out)
		// Expected configuration
		expected := map[string][]dummyConfiguration{
			"Modules": {
				{
					Module1: dummyModule1Configuration{
						Listen:  "127.0.0.1:8080",
						Topic:   "flows1",
						Workers: 100,
					},
					Module2: dummyModule2Configuration{
						MoreDetails: MoreDetails{
							Stuff: "hello",
						},
						Details: dummyModule2DetailsConfiguration{
							Workers:       1,
							IntervalValue: time.Minute,
						},
						Elements: []dummyModule2ElementsConfiguration{
							{
								Name:  "el1",
								Gauge: 10,
							}, {
								Name:  "el2",
								Gauge: 11,
							},
						},
					},
				}, {
					Module1: dummyModule1Configuration{
						Listen:  "127.0.0.1:8080",
						Topic:   "flows2",
						Workers: 100,
					},
					Module2: dummyModule2Configuration{
						MoreDetails: MoreDetails{
							Stuff: "hello",
						},
						Details: dummyModule2DetailsConfiguration{
							Workers:       1,
							IntervalValue: time.Minute,
						},
						Elements: []dummyModule2ElementsConfiguration{
							{
								Name:  "el1",
								Gauge: 10,
							}, {
								Name:  "el2",
								Gauge: 11,
							},
						},
					},
				},
			},
		}
		if diff := helpers.Diff(parsed, expected); diff != "" {
			t.Errorf("Parse() (-got, +want):\n%s", diff)
		}
	}
	t.Run("without pointer", func(t *testing.T) {
		try(t, func(c cmd.ConfigRelatedOptions, out *bytes.Buffer) interface{} {
			parsed := struct {
				Modules []dummyConfiguration
			}{}
			if err := c.Parse(out, "dummy", &parsed); err != nil {
				t.Fatalf("Parse() error:\n%+v", err)
			}
			return parsed
		})
	})
	t.Run("with pointer", func(t *testing.T) {
		try(t, func(c cmd.ConfigRelatedOptions, out *bytes.Buffer) interface{} {
			parsed := struct {
				Modules []*dummyConfiguration
			}{}
			if err := c.Parse(out, "dummy", &parsed); err != nil {
				t.Fatalf("Parse() error:\n%+v", err)
			}
			return parsed
		})
	})
}

func TestDevNullDefault(t *testing.T) {
	c := cmd.ConfigRelatedOptions{
		Path: "/dev/null",
		Dump: true,
	}

	var parsed dummyConfiguration
	out := bytes.NewBuffer([]byte{})
	if err := c.Parse(out, "dummy", &parsed); err != nil {
		t.Fatalf("Parse() error:\n%+v", err)
	}
	// Expected configuration
	expected := dummyConfiguration{
		Module1: dummyModule1Configuration{
			Listen:  "127.0.0.1:8080",
			Topic:   "nothingness",
			Workers: 100,
		},
		Module2: dummyModule2Configuration{
			MoreDetails: MoreDetails{
				Stuff: "hello",
			},
			Details: dummyModule2DetailsConfiguration{
				Workers:       1,
				IntervalValue: time.Minute,
			},
			Elements: []dummyModule2ElementsConfiguration{
				{
					Name:  "el1",
					Gauge: 10,
				}, {
					Name:  "el2",
					Gauge: 11,
				},
			},
		},
	}
	if diff := helpers.Diff(parsed, expected); diff != "" {
		t.Errorf("Parse() (-got, +want):\n%s", diff)
	}
}
