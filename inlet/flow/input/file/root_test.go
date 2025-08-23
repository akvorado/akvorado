// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package file

import (
	"net"
	"path"
	"sync"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/reporter"
)

func TestFileInput(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration().(*Configuration)
	configuration.Paths = []string{path.Join("testdata", "file1.txt"), path.Join("testdata", "file2.txt")}

	done := make(chan bool)
	expected := []*pb.RawFlow{
		{
			Payload:       []byte("hello world!\n"),
			SourceAddress: net.ParseIP("127.0.0.1").To16(),
		}, {
			Payload:       []byte("bye bye\n"),
			SourceAddress: net.ParseIP("127.0.0.1").To16(),
		}, {
			Payload:       []byte("hello world!\n"),
			SourceAddress: net.ParseIP("127.0.0.1").To16(),
		},
	}
	var mu sync.Mutex
	got := []*pb.RawFlow{}
	send := func(_ string, flow *pb.RawFlow) {
		// Make a copy
		payload := make([]byte, len(flow.Payload))
		copy(payload, flow.Payload)
		newFlow := pb.RawFlow{
			TimeReceived:  0,
			Payload:       payload,
			SourceAddress: flow.SourceAddress,
		}
		mu.Lock()
		if len(got) < len(expected) {
			got = append(got, &newFlow)
			if len(got) == len(expected) {
				close(done)
			}
		}
		mu.Unlock()
	}

	in, err := configuration.New(r, daemon.NewMock(t), send)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if err := in.Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	defer func() {
		if err := in.Stop(); err != nil {
			t.Fatalf("Stop() error:\n%+v", err)
		}
	}()

	select {
	case <-time.After(50 * time.Millisecond):
		t.Fatal("timeout while waiting to receive flows")
	case <-done:
		if diff := helpers.Diff(got, expected); diff != "" {
			t.Fatalf("Input data (-got, +want):\n%s", diff)
		}
	}
}
