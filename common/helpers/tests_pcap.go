// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package helpers

import (
	"bytes"
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

// ReadPcapL4 reads and parses a PCAP file and returns the payload (Layer 4). If
// there are several packets, they are concatenated.
func ReadPcapL4(t testing.TB, pcapfile string) []byte {
	t.Helper()
	source := readPcap(t, pcapfile)
	payload := bytes.NewBuffer([]byte{})
	for packet := range source.Packets() {
		payload.Write(packet.TransportLayer().LayerPayload())
	}
	return payload.Bytes()
}

// ReadPcapL2 reads and parses a PCAP file and returns the payload (Layer 2). If
// there are several packets, only the first one is returned.
func ReadPcapL2(t testing.TB, pcapfile string) []byte {
	t.Helper()
	source := readPcap(t, pcapfile)
	payload := bytes.NewBuffer([]byte{})
	for packet := range source.Packets() {
		payload.Write(packet.LinkLayer().LayerContents())
		payload.Write(packet.LinkLayer().LayerPayload())
		break
	}
	return payload.Bytes()
}
