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
)

func TestParseSocketControlMessage(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skip Linux-only test")
	}
	server, err := listenConfig.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
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
	for _, size := range []int{100, 1000, 10000, 100000, 1000000} {
		// Write a lot of messages to have some overflow.
		for i := 0; i < size; i++ {
			client.Write([]byte("hello"))
		}

		// Empty the queue
		payload := make([]byte, 1000)
		server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		for i := 0; i < size; i++ {
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
	client.Write([]byte("bye bye"))

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
	if oobMsg.Drops == 0 {
		t.Fatal("no drops detected")
	}
}

func TestSetReadBuffer(t *testing.T) {
	server, err := listenConfig.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket() error:\n%+v", err)
	}
	defer server.Close()

	client, err := net.Dial("udp", server.(*net.UDPConn).LocalAddr().String())
	if err != nil {
		t.Fatalf("Dial() error:\n%+v", err)
	}

	conn := client.(*net.UDPConn)
	if err := setReadBuffer(conn, 256); err != nil {
		t.Fatalf("setReadBuffer() error:\n%+v", err)
	}
}
