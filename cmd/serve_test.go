package cmd_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"gopkg.in/yaml.v2"

	"akvorado/cmd"
	"akvorado/helpers"
	"akvorado/kafka"
)

func TestServeDump(t *testing.T) {
	// Configuration file
	want := cmd.DefaultServeConfiguration
	want.HTTP.Listen = "127.0.0.1:8000"
	want.Flow.Listen = "0.0.0.0:2055"
	want.Flow.Workers = 2
	want.SNMP.Workers = 2
	want.SNMP.CacheDuration = 20 * time.Minute
	want.SNMP.DefaultCommunity = "private"
	want.Kafka.Topic = "netflow"
	want.Kafka.Version = kafka.Version(sarama.V2_8_1_0)
	want.Kafka.CompressionCodec = kafka.CompressionCodec(sarama.CompressionZSTD)
	want.Core.Workers = 3
	config := `---
http:
 listen: 127.0.0.1:8000
flow:
 listen: 0.0.0.0:2055
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
	var got cmd.ServeConfiguration
	if err := yaml.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error:\n%+v", err)
	}
	if diff := helpers.Diff(got, want); diff != "" {
		t.Errorf("`serve -D -C` (-got, +want):\n%s", diff)
	}
}

func TestServeEnvOverride(t *testing.T) {
	// Configuration file
	want := cmd.DefaultServeConfiguration
	want.HTTP.Listen = "127.0.0.1:8000"
	want.Flow.Listen = "0.0.0.0:2055"
	want.Flow.Workers = 3
	want.SNMP.Workers = 2
	want.SNMP.CacheDuration = 22 * time.Minute
	want.SNMP.DefaultCommunity = "privateer"
	want.Kafka.Topic = "netflow"
	want.Kafka.Version = kafka.Version(sarama.V2_8_1_0)
	want.Kafka.CompressionCodec = kafka.CompressionCodec(sarama.CompressionZSTD)
	want.Kafka.Brokers = []string{"127.0.0.1:9092", "127.0.0.2:9092"}
	want.Core.Workers = 3
	config := `---
http:
 listen: 127.0.0.1:8000
flow:
 listen: 0.0.0.0:2055
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
	os.Setenv("AKVORADO_FLOW_WORKERS", "3")
	os.Setenv("AKVORADO_SNMP_CACHEDURATION", "22m")
	os.Setenv("AKVORADO_SNMP_DEFAULTCOMMUNITY", "privateer")
	os.Setenv("AKVORADO_KAFKA_BROKERS", "127.0.0.1:9092,127.0.0.2:9092")

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
	var got cmd.ServeConfiguration
	if err := yaml.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error:\n%+v", err)
	}
	if diff := helpers.Diff(got, want); diff != "" {
		t.Errorf("`serve -D -C` (-got, +want):\n%s", diff)
	}
}
