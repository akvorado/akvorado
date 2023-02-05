// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"net"
	"path/filepath"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/flow/decoder"
	"akvorado/inlet/flow/decoder/netflow"
	"akvorado/inlet/flow/decoder/sflow"
)

// The goal is to benchmark flow decoding + encoding to protobuf

func BenchmarkDecodeEncodeNetflow(b *testing.B) {
	schema.DisableDebug(b)
	r := reporter.NewMock(b)
	sch := schema.NewMock(b)
	nfdecoder := netflow.New(r, decoder.Dependencies{Schema: sch})

	template := helpers.ReadPcapPayload(b, filepath.Join("decoder", "netflow", "testdata", "options-template-257.pcap"))
	got := nfdecoder.Decode(decoder.RawFlow{Payload: template, Source: net.ParseIP("127.0.0.1")})
	if got == nil || len(got) != 0 {
		b.Fatal("Decode() error on options template")
	}
	data := helpers.ReadPcapPayload(b, filepath.Join("decoder", "netflow", "testdata", "options-data-257.pcap"))
	got = nfdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
	if got == nil || len(got) != 0 {
		b.Fatal("Decode() error on options data")
	}
	template = helpers.ReadPcapPayload(b, filepath.Join("decoder", "netflow", "testdata", "template-260.pcap"))
	got = nfdecoder.Decode(decoder.RawFlow{Payload: template, Source: net.ParseIP("127.0.0.1")})
	if got == nil || len(got) != 0 {
		b.Fatal("Decode() error on template")
	}
	data = helpers.ReadPcapPayload(b, filepath.Join("decoder", "netflow", "testdata", "data-260.pcap"))

	for _, withEncoding := range []bool{true, false} {
		title := map[bool]string{
			true:  "with encoding",
			false: "without encoding",
		}[withEncoding]
		b.Run(title, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				got = nfdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
				if withEncoding {
					for _, flow := range got {
						sch.ProtobufMarshal(flow)
					}
				}
			}
			if got[0].ProtobufDebug != nil {
				b.Fatal("debug is enabled")
			}
		})
	}
}

func BenchmarkDecodeEncodeSflow(b *testing.B) {
	schema.DisableDebug(b)
	r := reporter.NewMock(b)
	sch := schema.NewMock(b)
	sdecoder := sflow.New(r, decoder.Dependencies{Schema: sch})
	data := helpers.ReadPcapPayload(b, filepath.Join("decoder", "sflow", "testdata", "data-1140.pcap"))

	for _, withEncoding := range []bool{true, false} {
		title := map[bool]string{
			true:  "with encoding",
			false: "without encoding",
		}[withEncoding]
		var got []*schema.FlowMessage
		b.Run(title, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				got = sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
				if withEncoding {
					for _, flow := range got {
						sch.ProtobufMarshal(flow)
					}
				}
			}
			if got[0].ProtobufDebug != nil {
				b.Fatal("debug is enabled")
			}
		})
	}
}
