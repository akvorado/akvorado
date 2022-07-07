// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/benbjohnson/clock"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestReceiveFlows(t *testing.T) {
	// UDP listener
	receiver, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	if err != nil {
		t.Fatalf("ListenUDP() error:\n%+v", err)
	}
	defer receiver.Close()

	// Flow generator
	r := reporter.NewMock(t)
	mockClock := clock.NewMock()
	config := DefaultConfiguration()
	config.Target = receiver.LocalAddr().String()
	config.Flows = []FlowConfiguration{
		{
			PerSecond:  1,
			InIfIndex:  10,
			OutIfIndex: 20,
			PeakHour:   21 * time.Hour,
			Multiplier: 1,
			SrcNet:     netip.MustParsePrefix("192.0.2.0/24"),
			DstNet:     netip.MustParsePrefix("203.0.113.0/24"),
			SrcAS:      65201,
			DstAS:      65202,
			SrcPort:    443,
			DstPort:    0,
			Protocol:   "tcp",
			Size:       1400,
		},
	}
	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
		Clock:  mockClock,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	mockClock.Set(time.Date(2022, 3, 15, 9, 14, 12, 0, time.UTC))
	helpers.StartStop(t, c)
	mockClock.Add(1 * time.Second)

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
	if len(got) != 2 {
		t.Errorf("Read() got %d packets, expected 2", len(got))
	}
	// We only check the headers. Decoding is already tested in nfdata_test.go.
	expected := []nfv9Header{
		{
			Version:        9,
			Count:          4,
			SystemUptime:   1,
			UnixSeconds:    1647335653,
			SequenceNumber: 1,
		}, {
			Version:        9,
			Count:          1,
			SystemUptime:   1,
			UnixSeconds:    1647335653,
			SequenceNumber: 2,
		},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Read() (-got, +want):\n%s", diff)
	}
}
