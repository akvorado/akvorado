// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"bytes"
	"fmt"
	"path"
	"runtime"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/inlet/flow/input/file"
	"akvorado/inlet/kafka"

	"github.com/IBM/sarama"
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

	producer, mockProducer := kafka.NewMock(t, r, kafka.DefaultConfiguration())
	done := make(chan bool)
	for i := range 100 {
		mockProducer.ExpectInputWithMessageCheckerFunctionAndSucceed(func(got *sarama.ProducerMessage) error {
			if i == 99 {
				defer close(done)
			}
			expected := sarama.ProducerMessage{
				Topic:     fmt.Sprintf("flows-v%d", pb.Version),
				Key:       got.Key,
				Value:     got.Value,
				Partition: got.Partition,
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("Send() (-got, +want):\n%s", diff)
			}
			val, _ := got.Value.Encode()
			if i%2 == 0 {
				if !bytes.Contains(val, []byte("hello world!")) {
					t.Fatalf("Send() did not return %q", "hello world!")
				}
			} else {
				if !bytes.Contains(val, []byte("bye bye")) {
					t.Fatalf("Send() did not return %q", "bye bye")
				}
			}
			return nil
		})
	}

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
	case <-time.After(time.Second):
		t.Fatalf("flows not received")
	}
}
