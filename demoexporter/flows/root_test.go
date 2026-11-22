// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// readHeaders reads the NetFlow packet headers already received.
func readHeaders(t *testing.T, receiver *net.UDPConn) []nfv9Header {
	t.Helper()
	receiver.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	got := []nfv9Header{}
	for {
		payload := make([]byte, 9000)
		_, err := receiver.Read(payload)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				break
			}
			t.Fatalf("Read() error:\n%+v", err)
		}
		header := nfv9Header{}
		if err := binary.Read(bytes.NewBuffer(payload), binary.BigEndian, &header); err != nil {
			t.Errorf("binary.Read() error:\n%+v", err)
		} else {
			got = append(got, header)
		}
	}
	return got
}

func TestReceiveFlows(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// UDP listener
		receiver, err := net.ListenUDP("udp", &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 0,
		})
		if err != nil {
			t.Fatalf("ListenUDP() error:\n%+v", err)
		}
		defer receiver.Close()

		// Flow generator. The rate guarantees at least one flow.
		r := reporter.NewMock(t)
		config := testConfiguration(t)
		config.Target = receiver.LocalAddr().String()
		config.Rate = 2.5
		// Send every flow to this instance.
		config.Mapping = []MappingRuleConfiguration{
			{Assign: []AssignConfiguration{{Exporter: "exporter1", Interface: 10, Weight: 1}}},
		}
		c, err := New(r, config, Dependencies{
			Daemon: daemon.NewMock(t),
		})
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
		helpers.StartStop(t, c)
		time.Sleep(1 * time.Second)

		got := readHeaders(t, receiver)
		if len(got) < 2 {
			t.Fatalf("Read() got %d packets, expected at least 2", len(got))
		}
		// The first packet holds the templates.
		expected := nfv9Header{
			Version:        9,
			Count:          4,
			SystemUptime:   1,
			UnixSeconds:    uint32(time.Now().Unix()),
			SequenceNumber: 1,
		}
		if diff := helpers.Diff(got[0], expected); diff != "" {
			t.Fatalf("Read() first packet (-got, +want):\n%s", diff)
		}
		// The others hold data.
		records := uint16(0)
		for _, header := range got[1:] {
			if diff := helpers.Diff(header.Version, uint16(9)); diff != "" {
				t.Fatalf("Read() data packet version (-got, +want):\n%s", diff)
			}
			records += header.Count
		}
		if records == 0 {
			t.Fatalf("Read() no flow record received")
		}
	})
}

func TestRedial(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		listen := func() *net.UDPConn {
			receiver, err := net.ListenUDP("udp", &net.UDPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: 0,
			})
			if err != nil {
				t.Fatalf("ListenUDP() error:\n%+v", err)
			}
			return receiver
		}

		receiver1 := listen()
		defer receiver1.Close()
		receiver2 := listen()
		defer receiver2.Close()

		// Flow generator
		r := reporter.NewMock(t)
		config := testConfiguration(t)
		config.Target = receiver1.LocalAddr().String()
		config.Rate = 2.5
		// Send every flow to this instance.
		config.Mapping = []MappingRuleConfiguration{
			{Assign: []AssignConfiguration{{Exporter: "exporter1", Interface: 10, Weight: 1}}},
		}
		c, err := New(r, config, Dependencies{
			Daemon: daemon.NewMock(t),
		})
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
		helpers.StartStop(t, c)

		// The initial target receives flows.
		time.Sleep(1 * time.Second)
		if got := readHeaders(t, receiver1); len(got) < 2 {
			t.Fatalf("readHeaders(receiver1) got %d packets, expected at least 2", len(got))
		}

		// Move the target. redial() reloads c.target on its next run
		// (every 30 ticks).
		newTarget := receiver2.LocalAddr().String()
		c.target.Store(&newTarget)
		time.Sleep(30 * time.Second)
		if got := readHeaders(t, receiver2); len(got) == 0 {
			t.Fatal("readHeaders(receiver2) got no packets, expected flows after redial")
		}
	})
}
