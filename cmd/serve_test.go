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
	want.SNMP.CacheDuration = 2 * time.Hour
	want.SNMP.DefaultCommunity = "private"
	want.Kafka.Topic = "netflow"
	want.Kafka.Version = kafka.Version(sarama.V2_8_1_0)
	want.Kafka.CompressionCodec = kafka.CompressionCodec(sarama.CompressionGZIP)
	want.Core.Workers = 3
	config := `---
http:
 listen: 127.0.0.1:8000
flow:
 listen: 0.0.0.0:2055
 workers: 2
snmp:
 workers: 2
 cacheduration: 2h
 defaultcommunity: private
kafka:
 topic: netflow
 compressioncodec: gzip
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
		t.Errorf("`serve -D -C` error:\n%+v", err)
	}
	var got cmd.ServeConfiguration
	if err := yaml.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error:\n%+v", err)
	}
	if diff := helpers.Diff(got, want); diff != "" {
		t.Errorf("`serve -D -C` (-got, +want):\n%s", diff)
	}
}
