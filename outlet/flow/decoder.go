// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"errors"
	"fmt"
	"net/netip"
	"runtime/debug"
	"time"

	"akvorado/common/pb"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"
	"akvorado/outlet/flow/decoder/netflow"
	"akvorado/outlet/flow/decoder/sflow"
)

// Decode decodes a raw flow from protobuf into flow messages.
func (c *Component) Decode(rawFlow *pb.RawFlow, bf *schema.FlowMessage, finalize decoder.FinalizeFlowFunc) error {
	// Get decoder directly by type
	dec, ok := c.decoders[rawFlow.Decoder]
	if !ok {
		return fmt.Errorf("decoder %v not available", rawFlow.Decoder)
	}

	// Convert pb.RawFlow to decoder.RawFlow
	sourceIP, ok := netip.AddrFromSlice(rawFlow.SourceAddress)
	if !ok {
		return errors.New("missing source address")
	}

	decoderInput := decoder.RawFlow{
		TimeReceived: time.Unix(int64(rawFlow.TimeReceived), 0),
		Payload:      rawFlow.Payload,
		Source:       sourceIP,
	}

	// Decode the flow
	options := decoder.Option{
		TimestampSource: rawFlow.TimestampSource,
	}

	if err := c.decodeWithMetrics(dec, decoderInput, options, bf, func() {
		if rawFlow.UseSourceAddress {
			bf.ExporterAddress = sourceIP
		}
		finalize()
	}); err != nil {
		return fmt.Errorf("failed to decode flow: %w", err)
	}

	return nil
}

// decodeWithMetrics wraps decoder calls with metrics tracking.
func (c *Component) decodeWithMetrics(dec decoder.Decoder, input decoder.RawFlow, options decoder.Option, bf *schema.FlowMessage, finalize decoder.FinalizeFlowFunc) error {
	defer func() {
		if r := recover(); r != nil {
			c.errLogger.Error().
				Str("decoder", dec.Name()).
				Str("panic", fmt.Sprint(r)).
				Str("stack", string(debug.Stack())).
				Msg("panic while decoding")
			c.metrics.decoderErrors.WithLabelValues(dec.Name()).Inc()
		}
	}()

	n, err := dec.Decode(input, options, bf, finalize)
	if err != nil {
		c.metrics.decoderErrors.WithLabelValues(dec.Name()).Inc()
		return err
	}
	c.metrics.decoderStats.WithLabelValues(dec.Name()).Add(float64(n))

	return nil
}

var availableDecoders = map[pb.RawFlow_Decoder]decoder.NewDecoderFunc{
	pb.RawFlow_DECODER_NETFLOW: netflow.New,
	pb.RawFlow_DECODER_SFLOW:   sflow.New,
}
