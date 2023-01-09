// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder_test

import (
	"testing"

	"github.com/golang/protobuf/proto"

	"akvorado/common/helpers"
	"akvorado/inlet/flow/decoder"
)

func TestProtoMarshalEmpty(t *testing.T) {
	flow := decoder.FlowMessage{}
	buf, err := flow.EncodeMessage()
	if err != nil {
		t.Fatalf("MarshalProto() error:\n%+v", err)
	}

	got := decoder.FlowMessage{}
	if err := got.DecodeMessage(buf); err != nil {
		t.Fatalf("DecodeMessage() error:\n%+v", err)
	}

	if diff := helpers.Diff(got, flow); diff != "" {
		t.Fatalf("MarshalProto() (-got, +want):\n%s", diff)
	}
}

func TestProtoMarshal(t *testing.T) {
	flow := decoder.FlowMessage{
		TimeReceived: 16999,
		SrcCountry:   "FR",
		DstCountry:   "US",
	}
	buf, err := flow.EncodeMessage()
	if err != nil {
		t.Fatalf("MarshalProto() error:\n%+v", err)
	}

	got := decoder.FlowMessage{}
	if err := got.DecodeMessage(buf); err != nil {
		t.Fatalf("DecodeMessage() error:\n%+v", err)
	}

	if diff := helpers.Diff(got, flow); diff != "" {
		t.Fatalf("MarshalProto() (-got, +want):\n%s", diff)
	}
}

func TestProtoMarshalBufferSizes(t *testing.T) {
	for cap := 0; cap < 100; cap++ {
		for len := 0; len <= cap; len++ {
			buf := make([]byte, len, cap)
			flow := decoder.FlowMessage{
				TimeReceived: 16999,
				SrcCountry:   "FR",
				DstCountry:   "US",
			}
			buf, err := flow.EncodeMessage()
			if err != nil {
				t.Fatalf("MarshalProto() error:\n%+v", err)
			}

			got := decoder.FlowMessage{}
			pbuf := proto.NewBuffer(buf)
			err = pbuf.DecodeMessage(&got)
			if err != nil {
				t.Fatalf("DecodeMessage() error:\n%+v", err)
			}

			if diff := helpers.Diff(got, flow); diff != "" {
				t.Fatalf("MarshalProto() (-got, +want):\n%s", diff)
			}
		}
	}
}
