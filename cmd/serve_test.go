package cmd_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"

	"akvorado/cmd"
	"akvorado/helpers"
)

func want(t *testing.T, got, expected interface{}) {
	t.Helper()
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Configuration (-got, +want):\n%s", diff)
	}
}

func TestServeDump(t *testing.T) {
	// Configuration file
	config := `---
http:
 listen: 127.0.0.1:8000
flow:
 inputs:
  - type: udp
    decoder: netflow
    listen: 0.0.0.0:2055
    workers: 5
 workers: 2
snmp:
 workers: 2
 cache-duration: 20m
 default-community: private
kafka:
 topic: netflow
 compression-codec: zstd
 version: 2.8.1
core:
 workers: 3
`
	configFile := filepath.Join(t.TempDir(), "akvorado.yaml")
	ioutil.WriteFile(configFile, []byte(config), 0644)

	// Start serves with it
	root := cmd.RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(os.Stderr)
	root.SetArgs([]string{"serve", "-D", "-C", "--config", configFile})
	cmd.ServeOptionsReset()
	err := root.Execute()
	if err != nil {
		t.Fatalf("`serve -D -C` error:\n%+v", err)
	}

	var got map[string]map[string]interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error:\n%+v", err)
	}
	want(t, got["flow"], map[string]interface{}{
		"inputs": []map[string]interface{}{{
			"type":      "udp",
			"decoder":   "netflow",
			"listen":    "0.0.0.0:2055",
			"queuesize": 100000,
			"workers":   5,
		}},
		"workers": 2,
	})
	want(t, got["snmp"]["workers"], 2)
	want(t, got["snmp"]["cacheduration"], "20m0s")
	want(t, got["snmp"]["defaultcommunity"], "private")
	want(t, got["kafka"]["topic"], "netflow")
	want(t, got["kafka"]["version"], "2.8.1")
	want(t, got["kafka"]["brokers"], []string{"127.0.0.1:9092"})
}

func TestServeEnvOverride(t *testing.T) {
	// Configuration file
	config := `---
http:
 listen: 127.0.0.1:8000
flow:
 inputs:
  - type: udp
    decoder: netflow
    listen: 0.0.0.0:2055
    workers: 5
 workers: 2
snmp:
 workers: 2
 cache-duration: 10m
kafka:
 topic: netflow
 compression-codec: zstd
 version: 2.8.1
core:
 workers: 3
`
	configFile := filepath.Join(t.TempDir(), "akvorado.yaml")
	ioutil.WriteFile(configFile, []byte(config), 0644)

	// Environment
	os.Setenv("AKVORADO_SNMP_CACHEDURATION", "22m")
	os.Setenv("AKVORADO_SNMP_DEFAULTCOMMUNITY", "privateer")
	os.Setenv("AKVORADO_KAFKA_BROKERS", "127.0.0.1:9092,127.0.0.2:9092")
	os.Setenv("AKVORADO_FLOW_WORKERS", "3")
	os.Setenv("AKVORADO_FLOW_INPUTS_0_LISTEN", "0.0.0.0:2056")
	// We may be lucky or the environment is keeping order
	os.Setenv("AKVORADO_FLOW_INPUTS_1_TYPE", "file")
	os.Setenv("AKVORADO_FLOW_INPUTS_1_DECODER", "netflow")
	os.Setenv("AKVORADO_FLOW_INPUTS_1_PATHS", "f1,f2")

	// Start serves with it
	root := cmd.RootCmd
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(os.Stderr)
	root.SetArgs([]string{"serve", "-D", "-C", "--config", configFile})
	cmd.ServeOptionsReset()
	err := root.Execute()
	if err != nil {
		t.Fatalf("`serve -D -C` error:\n%+v", err)
	}

	var got map[string]map[string]interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error:\n%+v", err)
	}
	want(t, got["snmp"]["cacheduration"], "22m0s")
	want(t, got["snmp"]["defaultcommunity"], "privateer")
	want(t, got["kafka"]["brokers"], []string{"127.0.0.1:9092", "127.0.0.2:9092"})
	want(t, got["flow"], map[string]interface{}{
		"inputs": []map[string]interface{}{
			{
				"type":      "udp",
				"decoder":   "netflow",
				"listen":    "0.0.0.0:2056",
				"queuesize": 100000,
				"workers":   5,
			}, {
				"type":    "file",
				"decoder": "netflow",
				"paths":   []string{"f1", "f2"},
			},
		},
		"workers": 3,
	})
}
