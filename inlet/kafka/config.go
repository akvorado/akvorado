// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"

	"akvorado/common/helpers"
	"akvorado/common/kafka"
)

// Configuration describes the configuration for the Kafka exporter.
type Configuration struct {
	kafka.Configuration `mapstructure:",squash" yaml:"-,inline"`
	// CompressionCodec defines the compression to use.
	CompressionCodec CompressionCodec
	// QueueSize defines the maximum number of messages to buffer.
	QueueSize int `validate:"min=1"`
}

// DefaultConfiguration represents the default configuration for the Kafka exporter.
func DefaultConfiguration() Configuration {
	return Configuration{
		Configuration:    kafka.DefaultConfiguration(),
		CompressionCodec: CompressionCodec(kgo.Lz4Compression()),
		QueueSize:        32,
	}
}

// CompressionCodec represents a compression codec.
type CompressionCodec kgo.CompressionCodec

// UnmarshalText produces a compression codec
func (cc *CompressionCodec) UnmarshalText(text []byte) error {
	codec := kgo.CompressionCodec{}
	switch string(text) {
	case "none":
		codec = kgo.NoCompression()
	case "gzip":
		codec = kgo.GzipCompression()
	case "snappy":
		codec = kgo.SnappyCompression()
	case "lz4":
		codec = kgo.Lz4Compression()
	case "zstd":
		codec = kgo.ZstdCompression()
	default:
		return fmt.Errorf("unknown compression codec: %s", text)
	}
	*cc = CompressionCodec(codec)
	return nil
}

// String turns a compression codec into a string
func (cc CompressionCodec) String() string {
	switch kgo.CompressionCodec(cc) {
	case kgo.NoCompression():
		return "none"
	case kgo.GzipCompression():
		return "gzip"
	case kgo.SnappyCompression():
		return "snappy"
	case kgo.Lz4Compression():
		return "lz4"
	case kgo.ZstdCompression():
		return "zstd"
	default:
		return "unknown"
	}
}

// MarshalText turns a compression codec into a string
func (cc CompressionCodec) MarshalText() ([]byte, error) {
	return []byte(cc.String()), nil
}

func init() {
	helpers.RegisterMapstructureDeprecatedFields[Configuration](
		"FlushInterval",   // bad for performance
		"FlushBytes",      //  duplicate with QueueSize
		"MaxMessageBytes", //  just tune QueueSize instead
	)
}
