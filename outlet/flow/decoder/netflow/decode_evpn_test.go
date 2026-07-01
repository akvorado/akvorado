package netflow

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"akvorado/common/constants"
	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

func readPcapLocal(t *testing.T, pcapfile string) *gopacket.PacketSource {
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

func TestDecodeEVPN(t *testing.T) {
	pcapfile := filepath.Join("testdata", "ethernet-over-mpls-with-control-word.pcap")
	source := readPcapLocal(t, pcapfile)
	var packets [][]byte
	for packet := range source.Packets() {
		udpLayer := packet.Layer(layers.LayerTypeUDP)
		if udpLayer != nil {
			packets = append(packets, udpLayer.LayerPayload())
		}
	}

	if len(packets) < 2 {
		t.Fatal("Not enough packets in PCAP")
	}

	_, nfdecoder, bf, got, finalize := setup(t, true)
	options := decoder.Options{}

	// Packet 2 has the template
	rawTemplate := decoder.RawFlow{
		Payload:      packets[1],
		Source:       netip.MustParseAddr("127.0.0.1"),
		TimeReceived: time.Now(),
	}
	_, err := nfdecoder.Decode(rawTemplate, options, bf, finalize)
	if err != nil {
		t.Fatalf("failed to decode template: %v", err)
	}

	// Packet 1 has the data
	rawData := decoder.RawFlow{
		Payload:      packets[0],
		Source:       netip.MustParseAddr("127.0.0.1"),
		TimeReceived: time.Now(),
	}
	_, err = nfdecoder.Decode(rawData, options, bf, finalize)
	if err != nil {
		t.Fatalf("failed to decode data: %v", err)
	}

	if len(*got) != 10 {
		t.Fatalf("expected 10 flows, got %d", len(*got))
	}

	// Verify Flow #4 (index 3), which has the EVPN MPLS packet including Control Word
	flow4 := (*got)[3]
	expectedFlow4 := &schema.FlowMessage{
		ExporterAddress: netip.MustParseAddr("127.0.0.1"),
		InIf:            1022,
		OutIf:           0,
		SrcAddr:         netip.MustParseAddr("::ffff:198.51.100.1"),
		DstAddr:         netip.MustParseAddr("::ffff:198.51.100.2"),
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnEType:         uint32(constants.ETypeIPv4),
			schema.ColumnProto:         uint32(constants.ProtoTCP),
			schema.ColumnSrcPort:       uint16(443),
			schema.ColumnDstPort:       uint16(55427),
			schema.ColumnTCPFlags:      uint16(16),
			schema.ColumnIPTTL:         uint8(62),
			schema.ColumnIPTos:         uint8(32),
			schema.ColumnIPFragmentID:  uint32(41037),
			schema.ColumnBytes:         uint64(1492),
			schema.ColumnPackets:       uint64(1),
			schema.ColumnMPLSLabels:    []uint32{300012, 17},
			schema.ColumnDstMAC:        uint64(0x020000000003),
			schema.ColumnSrcMAC:        uint64(0x020000000004),
			schema.ColumnFlowDirection: uint8(schema.DirectionIngress),
		},
	}

	if diff := helpers.Diff(flow4, expectedFlow4); diff != "" {
		t.Fatalf("Flow #4 diff (-got, +want):\n%s", diff)
	}
}
