// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cgnat

import (
	"testing"
	"time"
)

func TestParseSyslogLine(t *testing.T) {
	t.Run("allocated", func(t *testing.T) {
		event, err := ParseSyslogLine("Jul  6 14:05:36 host NAT:20260706140537 3e2d PortBatchV2Allocated: [100.104.128.32 62.45.100.176 11777 12288]")
		if err != nil {
			t.Fatalf("ParseSyslogLine() error:\n%+v", err)
		}
		if event.Operation != OperationAllocate {
			t.Fatalf("operation = %v, want %v", event.Operation, OperationAllocate)
		}
		if got, want := event.Timestamp, time.Date(2026, 7, 6, 14, 5, 37, 0, time.UTC); !got.Equal(want) {
			t.Fatalf("timestamp = %v, want %v", got, want)
		}
		if got, want := event.PrivateIP.String(), "100.104.128.32"; got != want {
			t.Fatalf("private IP = %s, want %s", got, want)
		}
		if got, want := event.PublicIP.String(), "62.45.100.176"; got != want {
			t.Fatalf("public IP = %s, want %s", got, want)
		}
		if got, want := event.PortStart, uint16(11777); got != want {
			t.Fatalf("start port = %d, want %d", got, want)
		}
		if got, want := event.PortEnd, uint16(12288); got != want {
			t.Fatalf("end port = %d, want %d", got, want)
		}
	})

	t.Run("freed", func(t *testing.T) {
		event, err := ParseSyslogLine("Jul  6 14:05:36 host NAT:20260706140537 3e2d PortBatchV2Freed: [100.75.192.4 62.45.100.58 17409 17920]")
		if err != nil {
			t.Fatalf("ParseSyslogLine() error:\n%+v", err)
		}
		if event.Operation != OperationFree {
			t.Fatalf("operation = %v, want %v", event.Operation, OperationFree)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		if _, err := ParseSyslogLine("no match"); err == nil {
			t.Fatal("expected parse error")
		}
	})
}

func TestEncodeDecode(t *testing.T) {
	event, err := ParseSyslogLine("Jul  6 14:05:36 host NAT:20260706140537 3e2d PortBatchV2Allocated: [100.104.128.32 62.45.100.176 11777 12288]")
	if err != nil {
		t.Fatalf("ParseSyslogLine() error:\n%+v", err)
	}

	payload, err := Encode(event)
	if err != nil {
		t.Fatalf("Encode() error:\n%+v", err)
	}

	decoded, err := Decode(payload)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}

	if decoded != event {
		t.Fatalf("decoded event mismatch:\n got: %+v\nwant: %+v", decoded, event)
	}
}
