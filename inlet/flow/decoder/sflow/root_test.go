// SPDX-FileCopyrightText: 2022 Tchadel Icard
// SPDX-License-Identifier: AGPL-3.0-only

package sflow

import (
	"net"
	"path/filepath"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/inlet/flow/decoder"
)

func TestDecode(t *testing.T) {
	r := reporter.NewMock(t)
	sdecoder := New(r)

	// Send data
	data := helpers.ReadPcapPayload(t, filepath.Join("testdata", "data-1140.pcap"))
	got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
	if got == nil {
		t.Fatalf("Decode() error on data")
	}
	expectedFlows := []*decoder.FlowMessage{
		{
			SequenceNum:     812646826,
			SamplingRate:    1024,
			TimeFlowStart:   18446744011573954816,
			TimeFlowEnd:     18446744011573954816,
			Bytes:           1518,
			Packets:         1,
			Etype:           0x86DD,
			Proto:           6,
			SrcPort:         46026,
			DstPort:         22,
			InIf:            27,
			OutIf:           28,
			IPTos:           8,
			IPTTL:           64,
			TCPFlags:        16,
			IPv6FlowLabel:   426132,
			SrcAddr:         net.ParseIP("2a0c:8880:2:0:185:21:130:38").To16(),
			DstAddr:         net.ParseIP("2a0c:8880:2:0:185:21:130:39").To16(),
			ExporterAddress: net.ParseIP("172.16.0.3").To16(),
		}, {
			SequenceNum:     812646826,
			SamplingRate:    1024,
			TimeFlowStart:   18446744011573954816,
			TimeFlowEnd:     18446744011573954816,
			Bytes:           439,
			Packets:         1,
			Etype:           0x800,
			Proto:           6,
			SrcPort:         443,
			DstPort:         56876,
			InIf:            49001,
			OutIf:           25,
			IPTTL:           59,
			TCPFlags:        24,
			FragmentId:      42354,
			FragmentOffset:  16384,
			SrcAS:           13335,
			DstAS:           39421,
			SrcNetMask:      20,
			DstNetMask:      27,
			SrcAddr:         net.ParseIP("104.26.8.24").To16(),
			DstAddr:         net.ParseIP("45.90.161.46").To16(),
			ExporterAddress: net.ParseIP("172.16.0.3").To16(),
			NextHop:         net.ParseIP("45.90.161.46").To16(),
		}, {
			SequenceNum:     812646826,
			SamplingRate:    1024,
			TimeFlowStart:   18446744011573954816,
			TimeFlowEnd:     18446744011573954816,
			Bytes:           1518,
			Packets:         1,
			Etype:           0x86DD,
			Proto:           6,
			SrcPort:         46026,
			DstPort:         22,
			InIf:            27,
			OutIf:           28,
			IPTos:           8,
			IPTTL:           64,
			TCPFlags:        16,
			IPv6FlowLabel:   426132,
			SrcAddr:         net.ParseIP("2a0c:8880:2:0:185:21:130:38").To16(),
			DstAddr:         net.ParseIP("2a0c:8880:2:0:185:21:130:39").To16(),
			ExporterAddress: net.ParseIP("172.16.0.3").To16(),
		}, {
			SequenceNum:     812646826,
			SamplingRate:    1024,
			TimeFlowStart:   18446744011573954816,
			TimeFlowEnd:     18446744011573954816,
			Bytes:           64,
			Packets:         1,
			Etype:           0x800,
			Proto:           6,
			SrcPort:         55658,
			DstPort:         5555,
			InIf:            28,
			OutIf:           49001,
			IPTTL:           255,
			TCPFlags:        2,
			FragmentId:      54321,
			SrcAS:           39421,
			DstAS:           26615,
			SrcNetMask:      27,
			DstNetMask:      17,
			SrcAddr:         net.ParseIP("45.90.161.148").To16(),
			DstAddr:         net.ParseIP("191.87.91.27").To16(),
			ExporterAddress: net.ParseIP("172.16.0.3").To16(),
			NextHop:         net.ParseIP("31.14.69.110").To16(),
			NextHopAS:       203698,
		}, {
			SequenceNum:     812646826,
			SamplingRate:    1024,
			TimeFlowStart:   18446744011573954816,
			TimeFlowEnd:     18446744011573954816,
			Bytes:           1518,
			Packets:         1,
			Etype:           0x86DD,
			Proto:           6,
			SrcPort:         46026,
			DstPort:         22,
			InIf:            27,
			OutIf:           28,
			IPTos:           8,
			IPTTL:           64,
			TCPFlags:        16,
			IPv6FlowLabel:   426132,
			SrcAddr:         net.ParseIP("2a0c:8880:2:0:185:21:130:38").To16(),
			DstAddr:         net.ParseIP("2a0c:8880:2:0:185:21:130:39").To16(),
			ExporterAddress: net.ParseIP("172.16.0.3").To16(),
		},
	}
	for _, f := range got {
		f.TimeReceived = 0
	}

	if diff := helpers.Diff(got, expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
	gotMetrics := r.GetMetrics(
		"akvorado_inlet_flow_decoder_sflow_",
		"count",
		"sample_",
	)
	expectedMetrics := map[string]string{
		`count{agent="172.16.0.3",exporter="127.0.0.1",version="5"}`:                                "1",
		`sample_records_sum{agent="172.16.0.3",exporter="127.0.0.1",type="FlowSample",version="5"}`: "14",
		`sample_sum{agent="172.16.0.3",exporter="127.0.0.1",type="FlowSample",version="5"}`:         "5",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
	}
}

func TestDecodeInterface(t *testing.T) {
	r := reporter.NewMock(t)
	sdecoder := New(r)

	t.Run("local interface", func(t *testing.T) {
		// Send data
		data := helpers.ReadPcapPayload(t, filepath.Join("testdata", "data-local-interface.pcap"))
		got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
		if got == nil {
			t.Fatalf("Decode() error on data")
		}
		expectedFlows := []*decoder.FlowMessage{
			{
				SequenceNum:     812646826,
				SamplingRate:    1024,
				TimeFlowStart:   18446744011573954816,
				TimeFlowEnd:     18446744011573954816,
				Bytes:           1518,
				Packets:         1,
				Etype:           0x86DD,
				Proto:           6,
				SrcPort:         46026,
				DstPort:         22,
				InIf:            27,
				OutIf:           0, // local interface
				IPTos:           8,
				IPTTL:           64,
				TCPFlags:        16,
				IPv6FlowLabel:   426132,
				SrcAddr:         net.ParseIP("2a0c:8880:2:0:185:21:130:38").To16(),
				DstAddr:         net.ParseIP("2a0c:8880:2:0:185:21:130:39").To16(),
				ExporterAddress: net.ParseIP("172.16.0.3").To16(),
			},
		}
		for _, f := range got {
			f.TimeReceived = 0
		}

		if diff := helpers.Diff(got, expectedFlows); diff != "" {
			t.Fatalf("Decode() (-got, +want):\n%s", diff)
		}
	})

	t.Run("discard interface", func(t *testing.T) {
		// Send data
		data := helpers.ReadPcapPayload(t, filepath.Join("testdata", "data-discard-interface.pcap"))
		got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
		if got == nil {
			t.Fatalf("Decode() error on data")
		}
		expectedFlows := []*decoder.FlowMessage{
			{
				SequenceNum:      812646826,
				SamplingRate:     1024,
				TimeFlowStart:    18446744011573954816,
				TimeFlowEnd:      18446744011573954816,
				Bytes:            1518,
				Packets:          1,
				Etype:            0x86DD,
				Proto:            6,
				SrcPort:          46026,
				DstPort:          22,
				InIf:             27,
				OutIf:            0, // discard interface
				ForwardingStatus: 128,
				IPTos:            8,
				IPTTL:            64,
				TCPFlags:         16,
				IPv6FlowLabel:    426132,
				SrcAddr:          net.ParseIP("2a0c:8880:2:0:185:21:130:38").To16(),
				DstAddr:          net.ParseIP("2a0c:8880:2:0:185:21:130:39").To16(),
				ExporterAddress:  net.ParseIP("172.16.0.3").To16(),
			},
		}
		for _, f := range got {
			f.TimeReceived = 0
		}

		if diff := helpers.Diff(got, expectedFlows); diff != "" {
			t.Fatalf("Decode() (-got, +want):\n%s", diff)
		}
	})

	t.Run("multiple interfaces", func(t *testing.T) {
		// Send data
		data := helpers.ReadPcapPayload(t, filepath.Join("testdata", "data-multiple-interfaces.pcap"))
		got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
		if got == nil {
			t.Fatalf("Decode() error on data")
		}
		expectedFlows := []*decoder.FlowMessage{
			{
				SequenceNum:     812646826,
				SamplingRate:    1024,
				TimeFlowStart:   18446744011573954816,
				TimeFlowEnd:     18446744011573954816,
				Bytes:           1518,
				Packets:         1,
				Etype:           0x86DD,
				Proto:           6,
				SrcPort:         46026,
				DstPort:         22,
				InIf:            27,
				OutIf:           0, // multiple interfaces
				IPTos:           8,
				IPTTL:           64,
				TCPFlags:        16,
				IPv6FlowLabel:   426132,
				SrcAddr:         net.ParseIP("2a0c:8880:2:0:185:21:130:38").To16(),
				DstAddr:         net.ParseIP("2a0c:8880:2:0:185:21:130:39").To16(),
				ExporterAddress: net.ParseIP("172.16.0.3").To16(),
			},
		}
		for _, f := range got {
			f.TimeReceived = 0
		}

		if diff := helpers.Diff(got, expectedFlows); diff != "" {
			t.Fatalf("Decode() (-got, +want):\n%s", diff)
		}
	})

	t.Run("expanded flow sample", func(t *testing.T) {
		// Send data
		data := helpers.ReadPcapPayload(t, filepath.Join("testdata", "data-sflow-expanded-sample.pcap"))
		got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
		if got == nil {
			t.Fatalf("Decode() error on data")
		}
		expectedFlows := []*decoder.FlowMessage{
			{
				SequenceNum:     115694180,
				SamplingRate:    1000,
				TimeFlowStart:   18446744011573954816,
				TimeFlowEnd:     18446744011573954816,
				Bytes:           126,
				Packets:         1,
				Etype:           2048,
				Proto:           6,
				SrcPort:         22,
				DstPort:         52237,
				InIf:            29001,
				OutIf:           1285816721,
				IPTos:           8,
				IPTTL:           61,
				TCPFlags:        24,
				FragmentId:      43854,
				FragmentOffset:  16384,
				SrcNetMask:      32,
				DstNetMask:      22,
				SrcAddr:         net.ParseIP("52.52.52.52").To16(),
				DstAddr:         net.ParseIP("53.53.53.53").To16(),
				ExporterAddress: net.ParseIP("49.49.49.49").To16(),
				NextHop:         net.ParseIP("54.54.54.54").To16(),
				NextHopAS:       8218,
				VlanID:          809,
				SrcAS:           203476,
				DstAS:           203361,
			},
		}
		for _, f := range got {
			f.TimeReceived = 0
		}

		if diff := helpers.Diff(got, expectedFlows); diff != "" {
			t.Fatalf("Decode() (-got, +want):\n%s", diff)
		}

	})
}

func TestDecodeInterfaceVLANs(t *testing.T) {
	r := reporter.NewMock(t)
	sdecoder := New(r)

	// Send data
	data := helpers.ReadPcapPayload(t, filepath.Join("testdata", "data-vlan-interfaces.pcap"))
	got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
	if got == nil {
		t.Fatalf("Decode() error on data")
	}
	expectedFlows := []*decoder.FlowMessage{
		{
			SequenceNum:     204609,
			SamplingRate:    1024,
			TimeFlowStart:   18446744011573954816,
			TimeFlowEnd:     18446744011573954816,
			Bytes:           1502,
			Packets:         1,
			Etype:           0x800,
			Proto:           6,
			SrcPort:         443,
			DstPort:         49683,
			InIf:            13,
			OutIf:           50,
			IPTTL:           63,
			TCPFlags:        16,
			FragmentId:      16466,
			FragmentOffset:  16384,
			VlanID:          131,
			SrcAddr:         net.ParseIP("23.62.157.110").To16(),
			DstAddr:         net.ParseIP("103.167.249.200").To16(),
			ExporterAddress: net.ParseIP("10.88.11.251").To16(),
		}, {
			SequenceNum:     204609,
			SamplingRate:    1024,
			TimeFlowStart:   18446744011573954816,
			TimeFlowEnd:     18446744011573954816,
			Bytes:           1492,
			Packets:         1,
			Etype:           0x800,
			Proto:           6,
			SrcPort:         443,
			DstPort:         60081,
			InIf:            19,
			OutIf:           49,
			IPTTL:           63,
			TCPFlags:        16,
			FragmentId:      36446,
			FragmentOffset:  16384,
			VlanID:          890,
			SrcAddr:         net.ParseIP("23.45.168.73").To16(),
			DstAddr:         net.ParseIP("103.167.249.36").To16(),
			ExporterAddress: net.ParseIP("10.88.11.251").To16(),
		}, {
			SequenceNum:     204609,
			SamplingRate:    1024,
			TimeFlowStart:   18446744011573954816,
			TimeFlowEnd:     18446744011573954816,
			Bytes:           1522,
			Packets:         1,
			Etype:           0x800,
			Proto:           6,
			SrcPort:         80,
			DstPort:         59230,
			InIf:            19,
			OutIf:           49,
			IPTTL:           62,
			TCPFlags:        16,
			FragmentId:      14683,
			FragmentOffset:  16384,
			VlanID:          890,
			SrcAddr:         net.ParseIP("203.134.13.48").To16(),
			DstAddr:         net.ParseIP("111.235.140.4").To16(),
			ExporterAddress: net.ParseIP("10.88.11.251").To16(),
		}, {
			SequenceNum:     204609,
			SamplingRate:    1024,
			TimeFlowStart:   18446744011573954816,
			TimeFlowEnd:     18446744011573954816,
			Bytes:           74,
			Packets:         1,
			Etype:           0x800,
			Proto:           6,
			SrcPort:         60350,
			DstPort:         80,
			InIf:            21,
			OutIf:           51,
			IPTTL:           62,
			IPTos:           160,
			TCPFlags:        16,
			FragmentId:      7225,
			FragmentOffset:  16384,
			VlanID:          3016,
			SrcAddr:         net.ParseIP("103.126.144.78").To16(),
			DstAddr:         net.ParseIP("203.134.13.7").To16(),
			ExporterAddress: net.ParseIP("10.88.11.251").To16(),
		}, {
			SequenceNum:     204609,
			SamplingRate:    1024,
			TimeFlowStart:   18446744011573954816,
			TimeFlowEnd:     18446744011573954816,
			Bytes:           1522,
			Packets:         1,
			Etype:           0x800,
			Proto:           6,
			SrcPort:         443,
			DstPort:         49410,
			InIf:            12,
			OutIf:           49,
			IPTTL:           50,
			TCPFlags:        16,
			FragmentId:      16486,
			FragmentOffset:  16384,
			VlanID:          120,
			SrcAddr:         net.ParseIP("185.180.14.234").To16(),
			DstAddr:         net.ParseIP("111.235.140.236").To16(),
			ExporterAddress: net.ParseIP("10.88.11.251").To16(),
		}, {
			SequenceNum:     204609,
			SamplingRate:    1024,
			TimeFlowStart:   18446744011573954816,
			TimeFlowEnd:     18446744011573954816,
			Bytes:           1522,
			Packets:         1,
			Etype:           0x800,
			Proto:           6,
			SrcPort:         80,
			DstPort:         59230,
			InIf:            19,
			OutIf:           49,
			IPTTL:           62,
			TCPFlags:        16,
			FragmentId:      14734,
			FragmentOffset:  16384,
			VlanID:          890,
			SrcAddr:         net.ParseIP("203.134.13.48").To16(),
			DstAddr:         net.ParseIP("111.235.140.4").To16(),
			ExporterAddress: net.ParseIP("10.88.11.251").To16(),
		}, {
			SequenceNum:     204609,
			SamplingRate:    1024,
			TimeFlowStart:   18446744011573954816,
			TimeFlowEnd:     18446744011573954816,
			Bytes:           1502,
			Packets:         1,
			Etype:           0x800,
			Proto:           6,
			SrcPort:         80,
			DstPort:         24604,
			InIf:            19,
			OutIf:           49,
			IPTTL:           63,
			TCPFlags:        24,
			FragmentId:      25972,
			FragmentOffset:  16384,
			VlanID:          890,
			SrcAddr:         net.ParseIP("23.45.168.201").To16(),
			DstAddr:         net.ParseIP("123.100.144.100").To16(),
			ExporterAddress: net.ParseIP("10.88.11.251").To16(),
		},
	}
	for _, f := range got {
		f.TimeReceived = 0
	}

	if diff := helpers.Diff(got, expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
	gotMetrics := r.GetMetrics(
		"akvorado_inlet_flow_decoder_sflow_",
		"count",
		"sample_",
	)
	expectedMetrics := map[string]string{
		`count{agent="10.88.11.251",exporter="127.0.0.1",version="5"}`:                                "1",
		`sample_records_sum{agent="10.88.11.251",exporter="127.0.0.1",type="FlowSample",version="5"}`: "7",
		`sample_sum{agent="10.88.11.251",exporter="127.0.0.1",type="FlowSample",version="5"}`:         "7",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
	}
}
