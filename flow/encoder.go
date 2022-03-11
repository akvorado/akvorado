package flow

import (
	"bytes"
	"encoding/json"
	"net"
)

type rawFlowMessage FlowMessage
type prettierFlowMessage struct {
	rawFlowMessage
	PrettierSrcAddr        string `json:"SrcAddr,omitempty"`
	PrettierDstAddr        string `json:"DstAddr,omitempty"`
	PrettierSamplerAddress string `json:"SamplerAddress,omitempty"`
}

// MarshalJSON marshals a flow message to JSON. It uses a textual
// format for IP addresses. This is expected to be used for debug
// purpose only.
func (fm FlowMessage) MarshalJSON() ([]byte, error) {
	prettier := prettierFlowMessage{
		rawFlowMessage:         rawFlowMessage(fm),
		PrettierSrcAddr:        net.IP(fm.SrcAddr).String(),
		PrettierDstAddr:        net.IP(fm.DstAddr).String(),
		PrettierSamplerAddress: net.IP(fm.SamplerAddress).String(),
	}
	prettier.SrcAddr = nil
	prettier.DstAddr = nil
	prettier.SamplerAddress = nil
	buf := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(&prettier); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
