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
	"testing/synctest"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

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

		// Flow generator
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		config.Target = receiver.LocalAddr().String()
		config.Flows = []FlowConfiguration{
			{
				PerSecond:  1,
				InIfIndex:  []int{10},
				OutIfIndex: []int{20},
				PeakHour:   21 * time.Hour,
				Multiplier: 1,
				SrcNet:     netip.MustParsePrefix("192.0.2.0/24"),
				DstNet:     netip.MustParsePrefix("203.0.113.0/24"),
				SrcAS:      []uint32{65201},
				DstAS:      []uint32{65202},
				SrcPort:    []uint16{443},
				Protocol:   []string{"tcp"},
				Size:       1400,
			},
		}
		c, err := New(r, config, Dependencies{
			Daemon: daemon.NewMock(t),
		})
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
		helpers.StartStop(t, c)
		time.Sleep(1 * time.Second)

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
				UnixSeconds:    uint32(time.Now().Unix()),
				SequenceNumber: 1,
			}, {
				Version:        9,
				Count:          1,
				SystemUptime:   1,
				UnixSeconds:    uint32(time.Now().Unix()),
				SequenceNumber: 2,
			},
		}
		if diff := helpers.Diff(got, expected); diff != "" {
			t.Fatalf("Read() (-got, +want):\n%s", diff)
		}
	})
}
