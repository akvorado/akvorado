// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

import (
	"bytes"
	"encoding/json"
	"net"
	"strings"
	"testing"

	"akvorado/common/helpers"
)

func TestJSONEncoding(t *testing.T) {
	flow := &FlowMessage{
		TimeReceived:    200,
		SequenceNum:     1000,
		SamplingRate:    1000,
		FlowDirection:   1,
		ExporterAddress: net.ParseIP("192.0.2.42"),
		TimeFlowStart:   100,
		TimeFlowEnd:     200,
		Bytes:           6765,
		Packets:         4,
		InIf:            300,
		OutIf:           200,
		SrcAddr:         net.ParseIP("67.43.156.77"),
		DstAddr:         net.ParseIP("2.125.160.216"),
		Etype:           0x800,
		Proto:           6,
		SrcPort:         8534,
		DstPort:         80,
		InIfProvider:    "Telia",
		InIfBoundary:    FlowMessage_EXTERNAL,
		OutIfBoundary:   FlowMessage_INTERNAL,
	}
	buf := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(buf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(flow); err != nil {
		t.Fatalf("Encode() error:\n%+v", err)
	}
	got := strings.Split(buf.String(), "\n")
	expected := strings.Split(`{
  "TimeReceived": 200,
  "SequenceNum": 1000,
  "SamplingRate": 1000,
  "FlowDirection": 1,
  "TimeFlowStart": 100,
  "TimeFlowEnd": 200,
  "Bytes": 6765,
  "Packets": 4,
  "Etype": 2048,
  "Proto": 6,
  "SrcPort": 8534,
  "DstPort": 80,
  "InIf": 300,
  "OutIf": 200,
  "InIfProvider": "Telia",
  "SrcAddr": "67.43.156.77",
  "DstAddr": "2.125.160.216",
  "ExporterAddress": "192.0.2.42",
  "InIfBoundary": "EXTERNAL",
  "OutIfBoundary": "INTERNAL"
}
`, "\n")
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Encode() (-got, +want):\n%s", diff)
	}
}
