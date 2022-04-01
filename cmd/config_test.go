package cmd_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"

	"akvorado/cmd"
	"akvorado/common/helpers"
)

func want(t *testing.T, got, expected interface{}) {
	t.Helper()
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Configuration (-got, +want):\n%s", diff)
	}
}

func TestDump(t *testing.T) {
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
snmp:
 workers: 2
 cache-duration: 20m
 default-community: private
kafka:
 connect:
  version: 2.8.1
  topic: netflow
 compression-codec: zstd
core:
 workers: 3
`
	configFile := filepath.Join(t.TempDir(), "akvorado.yaml")
	ioutil.WriteFile(configFile, []byte(config), 0644)

	c := cmd.ConfigRelatedOptions{
		Path: configFile,
		Dump: true,
	}
	conf := cmd.DefaultInletConfiguration
	buf := bytes.NewBuffer([]byte{})
	if err := c.Parse(buf, "inlet", conf); err != nil {
		t.Fatalf("Parse() error:\n%+v", err)
	}
	var got map[string]map[string]interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error:\n%+v", err)
	}
	want(t, got["flow"], map[string]interface{}{
		"inputs": []map[string]interface{}{{
			"type":          "udp",
			"decoder":       "netflow",
			"listen":        "0.0.0.0:2055",
			"queuesize":     100000,
			"receivebuffer": 0,
			"workers":       5,
		}},
	})
	want(t, got["snmp"]["workers"], 2)
	want(t, got["snmp"]["cacheduration"], "20m0s")
	want(t, got["snmp"]["defaultcommunity"], "private")
	want(t, got["kafka"]["connect"], map[string]interface{}{
		"brokers": []string{"127.0.0.1:9092"},
		"version": "2.8.1",
		"topic":   "netflow",
	})
}

func TestEnvOverride(t *testing.T) {
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
snmp:
 workers: 2
 cache-duration: 10m
kafka:
 connect:
  version: 2.8.1
  topic: netflow
 compression-codec: zstd
core:
 workers: 3
`
	configFile := filepath.Join(t.TempDir(), "akvorado.yaml")
	ioutil.WriteFile(configFile, []byte(config), 0644)

	// Environment
	os.Setenv("AKVORADO_INLET_SNMP_CACHEDURATION", "22m")
	os.Setenv("AKVORADO_INLET_SNMP_DEFAULTCOMMUNITY", "privateer")
	os.Setenv("AKVORADO_INLET_SNMP_WORKERS", "3")
	os.Setenv("AKVORADO_INLET_KAFKA_CONNECT_BROKERS", "127.0.0.1:9092,127.0.0.2:9092")
	os.Setenv("AKVORADO_INLET_FLOW_INPUTS_0_LISTEN", "0.0.0.0:2056")
	// We may be lucky or the environment is keeping order
	os.Setenv("AKVORADO_INLET_FLOW_INPUTS_1_TYPE", "file")
	os.Setenv("AKVORADO_INLET_FLOW_INPUTS_1_DECODER", "netflow")
	os.Setenv("AKVORADO_INLET_FLOW_INPUTS_1_PATHS", "f1,f2")

	c := cmd.ConfigRelatedOptions{
		Path: configFile,
		Dump: true,
	}
	conf := cmd.DefaultInletConfiguration
	buf := bytes.NewBuffer([]byte{})
	if err := c.Parse(buf, "inlet", conf); err != nil {
		t.Fatalf("Parse() error:\n%+v", err)
	}
	var got map[string]map[string]interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error:\n%+v", err)
	}
	want(t, got["snmp"]["cacheduration"], "22m0s")
	want(t, got["snmp"]["defaultcommunity"], "privateer")
	want(t, got["snmp"]["workers"], 3)
	want(t, got["kafka"]["connect"], map[string]interface{}{
		"brokers": []string{"127.0.0.1:9092", "127.0.0.2:9092"},
		"version": "2.8.1",
		"topic":   "netflow",
	})
	want(t, got["flow"], map[string]interface{}{
		"inputs": []map[string]interface{}{
			{
				"type":          "udp",
				"decoder":       "netflow",
				"listen":        "0.0.0.0:2056",
				"queuesize":     100000,
				"receivebuffer": 0,
				"workers":       5,
			}, {
				"type":    "file",
				"decoder": "netflow",
				"paths":   []string{"f1", "f2"},
			},
		},
	})
}
