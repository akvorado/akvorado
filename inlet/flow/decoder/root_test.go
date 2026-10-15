// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/pb"
)

func TestGet(t *testing.T) {
	for _, d := range []pb.RawFlow_Decoder{
		pb.RawFlow_DECODER_NETFLOW,
		pb.RawFlow_DECODER_SFLOW,
		pb.RawFlow_DECODER_GOB,
	} {
		dec, err := Get(d)
		if err != nil {
			t.Fatalf("Get(%s) error:\n%+v", d, err)
		}
		got, _ := d.MarshalText()
		if diff := helpers.Diff(dec.Name(), string(got)); diff != "" {
			t.Errorf("Get(%s).Name() (-got, +want):\n%s", d, diff)
		}
	}
	if _, err := Get(pb.RawFlow_DECODER_UNSPECIFIED); err == nil {
		t.Error("Get(DECODER_UNSPECIFIED) did not error")
	}
}

func TestSplitter(t *testing.T) {
	cases := []struct {
		Decoder  pb.RawFlow_Decoder
		Splitter bool
	}{
		{pb.RawFlow_DECODER_NETFLOW, true},
		{pb.RawFlow_DECODER_SFLOW, false},
		{pb.RawFlow_DECODER_GOB, false},
	}
	for _, tc := range cases {
		dec, _ := Get(tc.Decoder)
		_, ok := dec.(Splitter)
		if diff := helpers.Diff(ok, tc.Splitter); diff != "" {
			t.Errorf("Get(%s) is a Splitter (-got, +want):\n%s", tc.Decoder, diff)
		}
	}
}
