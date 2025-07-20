// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"bytes"
	"fmt"
	"path"
	"runtime"
	"sync"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	kafkaCommon "akvorado/common/kafka"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/inlet/flow/input/file"
	"akvorado/inlet/kafka"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestFlow(t *testing.T) {
	_, src, _, _ := runtime.Caller(0)
	base := path.Join(path.Dir(src), "input", "file", "testdata")
	paths := []string{
		path.Join(base, "file1.txt"),
		path.Join(base, "file2.txt"),
	}

	inputs := []InputConfiguration{
		{
			Config: &file.Configuration{
				Paths:    paths,
				MaxFlows: 100,
			},
		},
	}

	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	config.Inputs = inputs

	producer, cluster := kafka.NewMock(t, r, kafka.DefaultConfiguration())
	defer cluster.Close()

	// Use the new helper to intercept messages
	var mu sync.Mutex
	helloCount := 0
	byeCount := 0
	totalCount := 0
	done := make(chan bool)

	kafkaCommon.InterceptMessages(t, cluster, func(record *kgo.Record) {
		mu.Lock()
		defer mu.Unlock()

		// Check topic
		expectedTopic := fmt.Sprintf("flows-v%d", pb.Version)
		if record.Topic != expectedTopic {
			t.Errorf("Expected topic %s, got %s", expectedTopic, record.Topic)
			return
		}

		// Count messages based on content
		if bytes.Contains(record.Value, []byte("hello world!")) {
			helloCount++
		} else if bytes.Contains(record.Value, []byte("bye bye")) {
			byeCount++
		}

		totalCount++
		if totalCount >= 100 {
			close(done)
		}
	})

	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
		HTTP:   httpserver.NewMock(t, r),
		Kafka:  producer,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	// Wait for flows
	select {
	case <-done:
		// Check that we got the expected number of each message type
		mu.Lock()
		if helloCount != 50 {
			t.Errorf("Expected 50 'hello world!' messages, got %d", helloCount)
		}
		if byeCount != 50 {
			t.Errorf("Expected 50 'bye bye' messages, got %d", byeCount)
		}
		mu.Unlock()
	case <-time.After(time.Second):
		t.Fatalf("flows not received")
	}
}
