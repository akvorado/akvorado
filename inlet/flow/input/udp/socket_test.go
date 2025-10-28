// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package udp

import (
	"context"
	"errors"
	"net"
	"os"
	"runtime"
	"testing"
	"time"

	"akvorado/common/reporter"

	"golang.org/x/sys/unix"
)

func TestParseSocketControlMessage(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skip Linux-only test")
	}
	r := reporter.NewMock(t)
	server, err := listenConfig(r, udpSocketOptions, nil).
		ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket() error:\n%+v", err)
	}
	defer server.Close()

	client, err := net.Dial("udp", server.(*net.UDPConn).LocalAddr().String())
	if err != nil {
		t.Fatalf("Dial() error:\n%+v", err)
	}

	overflow := false
outer:
	for _, count := range []int{100, 1000, 10_000, 100_000, 1_000_000} {
		// Write a lot of messages to have some overflow.
		for range count {
			client.Write([]byte("hello"))
		}

		// Empty the queue
		payload := make([]byte, 1000)
		server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		for range count {
			_, _, err := server.ReadFrom(payload)
			if errors.Is(err, os.ErrDeadlineExceeded) {
				overflow = true
				break outer
			}
		}
	}
	if !overflow {
		t.Fatalf("unable to trigger an overflow")
	}

	// Write one extra message
	server.SetReadDeadline(time.Time{})
	if _, err := client.Write([]byte("bye bye")); err != nil {
		t.Fatalf("Write() error:\n%+v", err)
	}

	// Read it
	payload := make([]byte, 1000)
	oob := make([]byte, oobLength)
	n, oobn, _, _, err := server.(*net.UDPConn).ReadMsgUDP(payload, oob)
	if err != nil {
		t.Fatalf("ReadMsgUDP() error:\n%+v", err)
	}
	if string(payload[:n]) != "bye bye" {
		t.Errorf("ReadMsgUDP() (-got, +want):\n-%s\n+%s", string(payload[:n]), "hello")
	}

	oobMsg, err := parseSocketControlMessage(oob[:oobn])
	if err != nil {
		t.Fatalf("parseSocketControlMessage() error:\n%+v", err)
	}
	t.Logf("Drops: %d", oobMsg.Drops)
	if oobMsg.Drops == 0 {
		t.Fatal("no drops detected")
	}
	if oobMsg.Drops > 1_000_000 {
		t.Fatal("too many drops detected")
	}
}

func TestListenConfig(t *testing.T) {
	r := reporter.NewMock(t)

	t.Run("one mandatory option", func(t *testing.T) {
		_, err := listenConfig(r, []socketOption{
			{
				Name:      "SO_REUSEADDR",
				Level:     unix.SOL_SOCKET,
				Option:    unix.SO_REUSEADDR,
				Mandatory: true,
			},
		}, nil).ListenPacket(t.Context(), "udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("ListenPacket() error:\n%+v", err)
		}
	})

	t.Run("one mandatory not implemented option", func(t *testing.T) {
		_, err := listenConfig(r, []socketOption{
			{
				Name:      "SO_UNKNOWN",
				Level:     unix.SOL_SOCKET,
				Option:    9999,
				Mandatory: true,
			},
		}, nil).ListenPacket(t.Context(), "udp", "127.0.0.1:0")
		if err == nil {
			t.Fatal("ListenPacket() did not error error")
		}
	})

	t.Run("one optional not implemented option", func(t *testing.T) {
		_, err := listenConfig(r, []socketOption{
			{
				Name:   "SO_UNKNOWN",
				Level:  unix.SOL_SOCKET,
				Option: 9999,
			},
		}, nil).ListenPacket(t.Context(), "udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("ListenPacket() error:\n%+v", err)
		}
	})
}
