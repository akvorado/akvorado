package cmd_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/travisjeffery/jocko/jocko"
	"github.com/travisjeffery/jocko/jocko/config"
	"github.com/travisjeffery/jocko/protocol"
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

func TestServe(t *testing.T) {
	// Start a Kafka server
	kafkaServer, kafkaDir := jocko.NewTestServer(t, func(cfg *config.Config) {
		cfg.Bootstrap = true
		cfg.BootstrapExpect = 1
		cfg.StartAsLeader = true
	}, nil)
	defer os.RemoveAll(kafkaDir)
	if err := kafkaServer.Start(context.Background()); err != nil {
		t.Fatalf("kafkaServer.Start() error:\n%+v", err)
	}
	defer kafkaServer.Shutdown()

	// Create topic
	kafkaClient, err := jocko.Dial("tcp", kafkaServer.Addr().String())
	if err != nil {
		t.Fatalf("kafkaClient.Dial() error:\n%+v", err)
	}
	resp, err := kafkaClient.CreateTopics(&protocol.CreateTopicRequests{
		Requests: []*protocol.CreateTopicRequest{{
			Topic:             "flows",
			NumPartitions:     1,
			ReplicationFactor: 1,
		}},
	})
	if err != nil {
		t.Fatalf("kafkaClient.CreateTopics() error:\n%+v", err)
	}
	for _, topicErrCode := range resp.TopicErrorCodes {
		if topicErrCode.ErrorCode != protocol.ErrNone.Code() && topicErrCode.ErrorCode != protocol.ErrTopicAlreadyExists.Code() {
			err := protocol.Errs[topicErrCode.ErrorCode]
			t.Fatalf("kafkaClient.CreateTopics() error:\n%+v", err)
		}
	}

	// Configuration using the Kafka broker
	config := fmt.Sprintf(`---
http:
 listen: 127.0.0.1:0
flow:
 listen: 127.0.0.1:0
kafka:
 version: 0.10.0.1
 brokers:
  - %s
`, kafkaServer.Addr().String())
	configFile := filepath.Join(t.TempDir(), "akvorado.yaml")
	ioutil.WriteFile(configFile, []byte(config), 0644)

	// Start
	root := cmd.RootCmd
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.SetArgs([]string{"serve", "--config", configFile, "--stop-after", "50ms"})
	cmd.ServeOptionsReset()
	err = root.Execute()
	if err != nil {
		t.Errorf("`serve` error:\n%+v", err)
	}
}
