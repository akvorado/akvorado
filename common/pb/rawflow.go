// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package pb contains the definition of RawFlow, the protobuf-based
// structure to exchange flows between the inlet and the outlet.
package pb

import (
	"errors"
	"fmt"

	"akvorado/common/helpers/bimap"
)

// Version is the version of the schema. On incompatible changes, this should be
// bumped.
var Version = 5

var decoderMap = bimap.New(map[RawFlow_Decoder]string{
	RawFlow_DECODER_NETFLOW: "netflow",
	RawFlow_DECODER_SFLOW:   "sflow",
	RawFlow_DECODER_GOB:     "gob",
})

// MarshalText turns a decoder to text
func (d RawFlow_Decoder) MarshalText() ([]byte, error) {
	got, ok := decoderMap.LoadValue(d)
	if ok {
		return []byte(got), nil
	}
	return nil, fmt.Errorf("unknown decoder %d", d)
}

// UnmarshalText provides a decoder from text
func (d *RawFlow_Decoder) UnmarshalText(input []byte) error {
	if len(input) == 0 {
		*d = RawFlow_DECODER_UNSPECIFIED
		return nil
	}
	got, ok := decoderMap.LoadKey(string(input))
	if ok {
		*d = got
		return nil
	}
	return errors.New("unknown decoder")
}

var tsMap = bimap.New(map[RawFlow_TimestampSource]string{
	RawFlow_TS_INPUT:                  "input", // this is the default value
	RawFlow_TS_NETFLOW_FIRST_SWITCHED: "netflow-first-switched",
	RawFlow_TS_NETFLOW_PACKET:         "netflow-packet",
})

// MarshalText turns a timestamp source to text
func (ts RawFlow_TimestampSource) MarshalText() ([]byte, error) {
	got, ok := tsMap.LoadValue(ts)
	if ok {
		return []byte(got), nil
	}
	return nil, errors.New("unknown timestamp source")
}

// UnmarshalText provides a timestamp source from text
func (ts *RawFlow_TimestampSource) UnmarshalText(input []byte) error {
	if len(input) == 0 {
		*ts = RawFlow_TS_INPUT
		return nil
	}
	if string(input) == "udp" {
		*ts = RawFlow_TS_INPUT
		return nil
	}
	got, ok := tsMap.LoadKey(string(input))
	if ok {
		*ts = got
		return nil
	}
	return fmt.Errorf("unknown timestamp source %q", string(input))
}

var decapsulationMap = bimap.New(map[RawFlow_DecapsulationProtocol]string{
	RawFlow_DECAP_NONE:  "none",
	RawFlow_DECAP_IPIP:  "ipip",
	RawFlow_DECAP_GRE:   "gre",
	RawFlow_DECAP_VXLAN: "vxlan",
	RawFlow_DECAP_SRV6:  "srv6",
})

// MarshalText turns a timestamp source to text
func (dp RawFlow_DecapsulationProtocol) MarshalText() ([]byte, error) {
	got, ok := decapsulationMap.LoadValue(dp)
	if ok {
		return []byte(got), nil
	}
	return nil, errors.New("unknown decapsulation protocol")
}

// UnmarshalText provides a timestamp source from text
func (dp *RawFlow_DecapsulationProtocol) UnmarshalText(input []byte) error {
	got, ok := decapsulationMap.LoadKey(string(input))
	if ok {
		*dp = got
		return nil
	}
	return fmt.Errorf("unknown decapsulation protocol %q", string(input))
}
