// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"context"
	"net"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/inlet/flow/decoder"
	"akvorado/inlet/flow/decoder/netflow"
)

func TestGetNetflowData(t *testing.T) {
	r := reporter.NewMock(t)
	nfdecoder := netflow.New(r)

	ch := getNetflowTemplates(
		context.Background(),
		50,
		30000,
		time.Date(2022, 03, 15, 14, 33, 0, 0, time.UTC),
		time.Date(2022, 03, 15, 15, 33, 0, 0, time.UTC))
	got := []interface{}{}
	for payload := range ch {
		got = append(got, nfdecoder.Decode(decoder.RawFlow{
			Payload: payload, Source: net.ParseIP("127.0.0.1")}))
	}

	ch = getNetflowData(
		context.Background(),
		[]generatedFlow{
			{
				SrcAddr: net.ParseIP("192.0.2.206"),
				DstAddr: net.ParseIP("203.0.113.165"),
				EType:   0x800,
				IPFlow: IPFlow{
					Octets:        1500,
					Packets:       1,
					Proto:         6,
					SrcPort:       443,
					DstPort:       34974,
					InputInt:      10,
					OutputInt:     20,
					SrcAS:         65201,
					DstAS:         65202,
					ForwardStatus: 64,
					SrcMask:       24,
					DstMask:       23,
				},
			}, {
				SrcAddr: net.ParseIP("2001:db8::1"),
				DstAddr: net.ParseIP("2001:db8:2:0:cea5:d643:ec43:3772"),
				EType:   0x86dd,
				IPFlow: IPFlow{
					Octets:        1300,
					Packets:       1,
					Proto:         6,
					SrcPort:       33179,
					DstPort:       443,
					InputInt:      20,
					OutputInt:     10,
					SrcAS:         65201,
					DstAS:         65202,
					ForwardStatus: 64,
					SrcMask:       48,
					DstMask:       48,
				},
			}, {
				SrcAddr: net.ParseIP("192.0.2.236"),
				DstAddr: net.ParseIP("203.0.113.67"),
				EType:   0x800,
				IPFlow: IPFlow{
					Octets:        1339,
					Packets:       1,
					Proto:         6,
					SrcPort:       443,
					DstPort:       33199,
					InputInt:      10,
					OutputInt:     20,
					SrcAS:         65201,
					DstAS:         65202,
					ForwardStatus: 64,
					SrcMask:       24,
					DstMask:       24,
				},
			},
		},
		100,
		time.Date(2022, 03, 15, 14, 33, 0, 0, time.UTC),
		time.Date(2022, 03, 15, 16, 33, 0, 0, time.UTC))
	for payload := range ch {
		got = append(got, nfdecoder.Decode(decoder.RawFlow{
			Payload: payload, Source: net.ParseIP("127.0.0.1")}))
	}
	expected := []interface{}{
		[]interface{}{}, // templates
		[]interface{}{
			&decoder.FlowMessage{
				SequenceNum:      100,
				SamplingRate:     30000,
				ExporterAddress:  net.ParseIP("127.0.0.1"),
				TimeFlowStart:    1647361980,
				TimeFlowEnd:      1647361980,
				Bytes:            1500,
				Packets:          1,
				SrcAddr:          net.ParseIP("192.0.2.206"),
				DstAddr:          net.ParseIP("203.0.113.165"),
				Etype:            0x800,
				Proto:            6,
				SrcPort:          443,
				DstPort:          34974,
				InIf:             10,
				OutIf:            20,
				ForwardingStatus: 64,
				SrcAS:            65201,
				DstAS:            65202,
				SrcNet:           24,
				DstNet:           23,
			},
			&decoder.FlowMessage{
				SequenceNum:      100,
				SamplingRate:     30000,
				ExporterAddress:  net.ParseIP("127.0.0.1"),
				TimeFlowStart:    1647361980,
				TimeFlowEnd:      1647361980,
				Bytes:            1339,
				Packets:          1,
				SrcAddr:          net.ParseIP("192.0.2.236"),
				DstAddr:          net.ParseIP("203.0.113.67"),
				Etype:            0x800,
				Proto:            6,
				SrcPort:          443,
				DstPort:          33199,
				InIf:             10,
				OutIf:            20,
				ForwardingStatus: 64,
				SrcAS:            65201,
				DstAS:            65202,
				SrcNet:           24,
				DstNet:           24,
			},
		},
		[]interface{}{
			&decoder.FlowMessage{
				SequenceNum:      101,
				SamplingRate:     30000,
				ExporterAddress:  net.ParseIP("127.0.0.1"),
				TimeFlowStart:    1647361980,
				TimeFlowEnd:      1647361980,
				Bytes:            1300,
				Packets:          1,
				SrcAddr:          net.ParseIP("2001:db8::1"),
				DstAddr:          net.ParseIP("2001:db8:2:0:cea5:d643:ec43:3772"),
				Etype:            0x86dd,
				Proto:            6,
				SrcPort:          33179,
				DstPort:          443,
				InIf:             20,
				OutIf:            10,
				ForwardingStatus: 64,
				SrcAS:            65201,
				DstAS:            65202,
				SrcNet:           48,
				DstNet:           48,
			},
		},
	}
	for idx1 := range got {
		if got[idx1] == nil {
			continue
		}
		switch g := got[idx1].(type) {
		case []*decoder.FlowMessage:
			for idx2 := range g {
				g[idx2].TimeReceived = 0
			}
		}
	}

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("getNetflowData() (-got, +want):\n%s", diff)
	}
}
