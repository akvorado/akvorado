// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/helpers/yaml"
	"akvorado/common/reporter"
)

func TestOrchestratorStart(t *testing.T) {
	r := reporter.NewMock(t)
	config := OrchestratorConfiguration{}
	config.Reset()
	if err := orchestratorStart(r, config, true); err != nil {
		t.Fatalf("orchestratorStart() error:\n%+v", err)
	}
}

func TestOrchestratorConfig(t *testing.T) {
	tests, err := os.ReadDir("testdata/configurations")
	if err != nil {
		t.Fatalf("ReadDir(%q) error:\n%+v", "testdata/configurations", err)
	}
	for _, test := range tests {
		if !test.IsDir() {
			continue
		}
		t.Run(test.Name(), func(t *testing.T) {
			expected, err := os.ReadFile(
				filepath.Join("testdata/configurations", test.Name(), "expected.yaml"))
			if err != nil {
				t.Fatalf("ReadFile() error:\n%+v", err)
			}
			var expectedYAML struct {
				Paths map[string]any `yaml:"paths"`
			}
			if err := yaml.Unmarshal(expected, &expectedYAML); err != nil {
				t.Fatalf("yaml.Unmarshal(expected) error:\n%+v", err)
			}
			root := RootCmd
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetArgs([]string{
				"orchestrator", "--dump", "--check",
				filepath.Join("testdata/configurations", test.Name(), "in.yaml"),
			})
			if err := root.Execute(); err != nil {
				t.Fatalf("`orchestrator` command error:\n%+v", err)
			}
			var gotYAML map[string]any
			if err := yaml.Unmarshal(buf.Bytes(), &gotYAML); err != nil {
				t.Fatalf("yaml.Unmarshal(output) error:\n%+v", err)
			}
			for path, expected := range expectedYAML.Paths {
				var got any
				got = gotYAML
				i := 0
				for _, component := range strings.Split(path, ".") {
					var ok bool
					i++
					switch gotConcrete := got.(type) {
					case []any:
						index, err := strconv.Atoi(component)
						if err != nil {
							t.Fatalf("key %q at level %d should be an int", path, i)
						}
						got = gotConcrete[index]
					case map[any]any:
						got, ok = gotConcrete[component]
						if !ok {
							t.Fatalf("key %q does not exist in result", path)
						}
					case map[string]any:
						got, ok = gotConcrete[component]
						if !ok {
							t.Fatalf("key %q does not exist in result", path)
						}
					default:
						t.Fatalf("key %q lead to unexpected type %v at level %d",
							path, reflect.TypeOf(got), i)
					}
				}
				if diff := helpers.Diff(got, expected); diff != "" {
					t.Fatalf("`orchestrator` --dump, key %q (-got, +want):\n%s", path, diff)
				}
			}
		})
	}
}

func TestOrchestrator(t *testing.T) {
	root := RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"orchestrator", "--check", "/dev/null"})
	err := root.Execute()
	if err != nil {
		t.Errorf("`orchestrator` error:\n%+v", err)
	}
}
