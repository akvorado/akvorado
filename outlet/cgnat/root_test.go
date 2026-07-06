// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cgnat

import (
	"testing"
	"time"

	commoncgnat "akvorado/common/cgnat"
	"akvorado/common/daemon"
	"akvorado/common/reporter"
)

func TestUpdateAndLookup(t *testing.T) {
	r := reporter.NewMock(t)
	c, err := New(r, DefaultConfiguration(), Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	allocated, err := commoncgnat.ParseSyslogLine("Jul  6 14:05:37 host NAT:20260706140537 3e2d PortBatchV2Allocated: [100.104.128.32 62.45.100.176 11777 12288]")
	if err != nil {
		t.Fatalf("ParseSyslogLine() error:\n%+v", err)
	}
	c.Update(allocated)

	if _, ok := c.Lookup(allocated.Timestamp, allocated.PublicIP, 11776); ok {
		t.Fatal("unexpected match below range")
	}
	match, ok := c.Lookup(allocated.Timestamp, allocated.PublicIP, 11777)
	if !ok {
		t.Fatal("expected range start to match")
	}
	if got, want := match.PrivateIP, allocated.PrivateIP; got != want {
		t.Fatalf("private IP = %s, want %s", got, want)
	}
	if _, ok := c.Lookup(allocated.Timestamp, allocated.PublicIP, 12288); !ok {
		t.Fatal("expected range end to match")
	}

	freed, err := commoncgnat.ParseSyslogLine("Jul  6 14:08:37 host NAT:20260706140837 3e2d PortBatchV2Freed: [100.104.128.32 62.45.100.176 11777 12288]")
	if err != nil {
		t.Fatalf("ParseSyslogLine() error:\n%+v", err)
	}
	c.Update(freed)

	if _, ok := c.Lookup(freed.Timestamp.Add(time.Second), freed.PublicIP, 12000); ok {
		t.Fatal("expected lookup miss after free")
	}
}
