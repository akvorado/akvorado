// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"testing"

	"github.com/Shopify/sarama"
)

func TestCompressionCodecUnmarshal(t *testing.T) {
	cases := []struct {
		Input         string
		Expected      sarama.CompressionCodec
		ExpectedError bool
	}{
		{"none", sarama.CompressionNone, false},
		{"zstd", sarama.CompressionZSTD, false},
		{"gzip", sarama.CompressionGZIP, false},
		{"unknown", sarama.CompressionNone, true},
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
		if cmp != CompressionCodec(tc.Expected) {
			t.Errorf("UnmarshalText(%q) got %v but expected %v", tc.Input, cmp, tc.Expected)
			continue
		}
	}
}
