// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package helpers

import (
	"bytes"
	"iter"
	"os"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

// readPcap reads and parse a PCAP file.
func readPcap(t testing.TB, pcapfile string) *gopacket.PacketSource {
	t.Helper()
	f, err := os.Open(pcapfile)
	if err != nil {
		t.Fatalf("Open(%q) error:\n%+v", pcapfile, err)
	}
	t.Cleanup(func() {
		f.Close()
	})

	reader, err := pcapgo.NewReader(f)
	if err != nil {
		t.Fatalf("NewReader(%q) error:\n%+v", pcapfile, err)
	}
	return gopacket.NewPacketSource(reader, layers.LayerTypeEthernet)
}

// ReadPcapL4 reads and parses a PCAP file and returns the payload (Layer 4).
// Several packets are only allowed for a single TCP stream. In this case, they
// are concatenated.
func ReadPcapL4(t testing.TB, pcapfile string) []byte {
	t.Helper()
	source := readPcap(t, pcapfile)
	payload := bytes.NewBuffer([]byte{})
	var first gopacket.Packet
	for packet := range source.Packets() {
		if first == nil {
			first = packet
		} else if first.TransportLayer().LayerType() != layers.LayerTypeTCP ||
			packet.TransportLayer().LayerType() != layers.LayerTypeTCP {
			t.Fatalf("%q contains more than one non-TCP packet", pcapfile)
		} else if first.NetworkLayer().NetworkFlow() != packet.NetworkLayer().NetworkFlow() ||
			first.TransportLayer().TransportFlow() != packet.TransportLayer().TransportFlow() {
			t.Fatalf("%q contains more than one TCP stream", pcapfile)
		}
		payload.Write(packet.TransportLayer().LayerPayload())
	}
	return payload.Bytes()
}

// ReadManyPcapL4 reads and parses a PCAP file and returns an iterator over the
// payload (Layer 4) of each packet.
func ReadManyPcapL4(t testing.TB, pcapfile string) iter.Seq[[]byte] {
	t.Helper()
	source := readPcap(t, pcapfile)
	return func(yield func([]byte) bool) {
		for packet := range source.Packets() {
			if !yield(packet.TransportLayer().LayerPayload()) {
				return
			}
		}
	}
}

// ReadPcapL2 reads and parses a PCAP file and returns the payload (Layer 2).
// Only one packet is allowed in pcap.
func ReadPcapL2(t testing.TB, pcapfile string) []byte {
	t.Helper()
	source := readPcap(t, pcapfile)
	payload := bytes.NewBuffer([]byte{})
	count := 0
	for packet := range source.Packets() {
		if count > 0 {
			t.Fatalf("%q contains more than one packet", pcapfile)
		}
		payload.Write(packet.LinkLayer().LayerContents())
		payload.Write(packet.LinkLayer().LayerPayload())
		count++
	}
	return payload.Bytes()
}
