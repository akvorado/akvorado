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
	for _, c := range cases {
		var cmp CompressionCodec
		err := cmp.UnmarshalText([]byte(c.Input))
		if err != nil && !c.ExpectedError {
			t.Errorf("UnmarshalText(%q) error:\n%+v", c.Input, err)
			continue
		}
		if err == nil && c.ExpectedError {
			t.Errorf("UnmarshalText(%q) got %v but expected error", c.Input, cmp)
			continue
		}
		if cmp != CompressionCodec(c.Expected) {
			t.Errorf("UnmarshalText(%q) got %v but expected %v", c.Input, cmp, c.Expected)
			continue
		}
	}
}
