// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers_test

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/inlet/flow/decoder"

	"github.com/golang/protobuf/proto"
)

func TestProtoMarshalEmpty(t *testing.T) {
	var err error
	flow := decoder.FlowMessage{}
	buf := []byte{}
	buf, err = helpers.MarshalProto(buf, &flow)
	if err != nil {
		t.Fatalf("MarshalProto() error:\n%+v", err)
	}

	// Use old API to check
	got := decoder.FlowMessage{}
	pbuf := proto.NewBuffer(buf)
	err = pbuf.DecodeMessage(&got)

	if diff := helpers.Diff(got, flow); diff != "" {
		t.Fatalf("MarshalProto() (-got, +want):\n%s", diff)
	}
}

func TestProtoMarshal(t *testing.T) {
	var err error
	flow := decoder.FlowMessage{
		TimeReceived: 16999,
		SrcCountry:   "FR",
		DstCountry:   "US",
	}
	buf := []byte{}
	buf, err = helpers.MarshalProto(buf, &flow)
	if err != nil {
		t.Fatalf("MarshalProto() error:\n%+v", err)
	}

	// Use old API to check
	got := decoder.FlowMessage{}
	pbuf := proto.NewBuffer(buf)
	err = pbuf.DecodeMessage(&got)

	if diff := helpers.Diff(got, flow); diff != "" {
		t.Fatalf("MarshalProto() (-got, +want):\n%s", diff)
	}
}

func TestProtoMarshalBufferSizes(t *testing.T) {
	for cap := 0; cap < 100; cap++ {
		for len := 0; len <= cap; len++ {
			buf := make([]byte, len, cap)
			var err error
			flow := decoder.FlowMessage{
				TimeReceived: 16999,
				SrcCountry:   "FR",
				DstCountry:   "US",
			}
			buf, err = helpers.MarshalProto(buf, &flow)
			if err != nil {
				t.Fatalf("MarshalProto() error:\n%+v", err)
			}

			// Use old API to check
			got := decoder.FlowMessage{}
			pbuf := proto.NewBuffer(buf)
			err = pbuf.DecodeMessage(&got)

			if diff := helpers.Diff(got, flow); diff != "" {
				t.Fatalf("MarshalProto() (-got, +want):\n%s", diff)
			}
		}
	}
}
