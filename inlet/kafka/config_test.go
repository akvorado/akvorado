// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"testing"

	"akvorado/common/helpers"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestCompressionCodecUnmarshal(t *testing.T) {
	cases := []struct {
		Input         string
		Expected      kgo.CompressionCodec
		ExpectedError bool
	}{
		{"none", kgo.NoCompression(), false},
		{"zstd", kgo.ZstdCompression(), false},
		{"gzip", kgo.GzipCompression(), false},
		{"snappy", kgo.SnappyCompression(), false},
		{"lz4", kgo.Lz4Compression(), false},
		{"unknown", kgo.NoCompression(), true},
	}
	for _, tc := range cases {
		var cmp CompressionCodec
		err := cmp.UnmarshalText([]byte(tc.Input))
		if err != nil && !tc.ExpectedError {
			t.Errorf("UnmarshalText(%q) error:\n%+v", tc.Input, err)
			continue
		}
		if err == nil && tc.ExpectedError {
			t.Errorf("UnmarshalText(%q) got %v but expected error", tc.Input, cmp)
			continue
		}
		if !tc.ExpectedError && cmp != CompressionCodec(tc.Expected) {
			t.Errorf("UnmarshalText(%q) got %v but expected %v", tc.Input, cmp, tc.Expected)
			continue
		}
		if !tc.ExpectedError && cmp.String() != tc.Input {
			t.Errorf("String() got %q but expected %q", cmp.String(), tc.Input)
		}
	}
}

func TestDefaultConfiguration(t *testing.T) {
	if err := helpers.Validate.Struct(DefaultConfiguration()); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}
