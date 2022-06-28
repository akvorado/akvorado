// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package decoder

import (
	"bytes"
	"encoding/json"
	"net"
)

type rawFlowMessage FlowMessage
type prettierFlowMessage struct {
	rawFlowMessage
	PrettierSrcAddr         string `json:"SrcAddr,omitempty"`
	PrettierDstAddr         string `json:"DstAddr,omitempty"`
	PrettierExporterAddress string `json:"ExporterAddress,omitempty"`
	PrettierInIfBoundary    string `json:"InIfBoundary,omitempty"`
	PrettierOutIfBoundary   string `json:"OutIfBoundary,omitempty"`
}

// MarshalJSON marshals a flow message to JSON. It uses a textual
// format for IP addresses. This is expected to be used for debug
// purpose only.
func (fm FlowMessage) MarshalJSON() ([]byte, error) {
	prettier := prettierFlowMessage{
		rawFlowMessage:          rawFlowMessage(fm),
		PrettierSrcAddr:         net.IP(fm.SrcAddr).String(),
		PrettierDstAddr:         net.IP(fm.DstAddr).String(),
		PrettierExporterAddress: net.IP(fm.ExporterAddress).String(),
		PrettierInIfBoundary:    fm.InIfBoundary.String(),
		PrettierOutIfBoundary:   fm.OutIfBoundary.String(),
	}
	prettier.SrcAddr = nil
	prettier.DstAddr = nil
	prettier.ExporterAddress = nil
	buf := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(&prettier); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
