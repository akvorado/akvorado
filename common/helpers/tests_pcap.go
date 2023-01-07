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

// ReadPcapPayload reads and parses a PCAP file and return the payload (after Layer 4).
func ReadPcapPayload(t testing.TB, pcapfile string) []byte {
	t.Helper()
	f, err := os.Open(pcapfile)
	if err != nil {
		t.Fatalf("Open(%q) error:\n%+v", pcapfile, err)
	}
	defer f.Close()

	reader, err := pcapgo.NewReader(f)
	if err != nil {
		t.Fatalf("NewReader(%q) error:\n%+v", pcapfile, err)
	}
	payload := bytes.NewBuffer([]byte{})
	source := gopacket.NewPacketSource(reader, layers.LayerTypeEthernet)
	for packet := range source.Packets() {
		payload.Write(packet.TransportLayer().LayerPayload())
	}
	return payload.Bytes()
}
