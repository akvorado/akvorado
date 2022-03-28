package udp

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestParseSocketControlMessage(t *testing.T) {
	server, err := listenConfig.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket() error:\n%+v", err)
	}
	defer server.Close()

	client, err := net.Dial("udp", server.(*net.UDPConn).LocalAddr().String())
	if err != nil {
		t.Fatalf("Dial() error:\n%+v", err)
	}

	// Write a lot of messages to have some overflow.
	for i := 0; i < 10000; i++ {
		client.Write([]byte("hello"))
	}

	// Empty the queue
	payload := make([]byte, 1000)
	server.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	for i := 0; i < 10000; i++ {
		server.ReadFrom(payload)
	}

	// Write one extra message
	server.SetReadDeadline(time.Time{})
	client.Write([]byte("bye bye"))

	// Read it
	oob := make([]byte, oobLength)
	n, oobn, _, _, err := server.(*net.UDPConn).ReadMsgUDP(payload, oob)
	if err != nil {
		t.Fatalf("ReadMsgUDP() error:\n%+v", err)
	}
	if string(payload[:n]) != "bye bye" {
		t.Errorf("ReadMsgUDP() (-got, +want):\n-%s\n+%s", string(payload[:n]), "hello")
	}

	drops, err := parseSocketControlMessage(oob[:oobn])
	if err != nil {
		t.Fatalf("parseSocketControlMessage() error:\n%+v", err)
	}
	if drops == 0 {
		t.Fatal("no drops detected")
	}
}
