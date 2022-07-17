// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package sflow

import (
	"io/ioutil"
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
	data, err := ioutil.ReadFile(filepath.Join("testdata", "data-1140.data"))
	if err != nil {
		panic(err)
	}
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
			SrcNet:          20,
			DstNet:          27,
			SrcAddr:         net.ParseIP("104.26.8.24").To16(),
			DstAddr:         net.ParseIP("45.90.161.46").To16(),
			ExporterAddress: net.ParseIP("172.16.0.3").To16(),
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
			SrcNet:          27,
			DstNet:          17,
			SrcAddr:         net.ParseIP("45.90.161.148").To16(),
			DstAddr:         net.ParseIP("191.87.91.27").To16(),
			ExporterAddress: net.ParseIP("172.16.0.3").To16(),
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
