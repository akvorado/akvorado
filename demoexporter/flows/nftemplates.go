// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"bytes"
	"context"
	"encoding/binary"
	"time"

	"akvorado/common/helpers"

	"github.com/netsampler/goflow2/v2/decoders/netflow"
)

type flowFamilySettings struct {
	MaxFlowsPerPacket int
	FlowLength        int
	TemplateID        uint16
	Template          []templateField
}

var flowSettings = map[uint16]*flowFamilySettings{
	helpers.ETypeIPv4: {
		TemplateID: 260,
	},
	helpers.ETypeIPv6: {
		TemplateID: 261,
	},
}

const optionsTemplateID = 262

// IPFlow represents an IP flow (without the IP-dependant part)
type IPFlow struct {
	Packets       uint32
	Octets        uint32
	InputInt      uint32
	OutputInt     uint32
	StartTime     uint32
	EndTime       uint32
	SrcPort       uint16
	DstPort       uint16
	SrcAS         uint32
	DstAS         uint32
	Proto         uint8
	ForwardStatus uint8
	SamplerID     uint16
	SrcMask       uint8
	DstMask       uint8
}

var ipTemplate = []templateField{
	{netflow.NFV9_FIELD_IN_PKTS, 4},
	{netflow.NFV9_FIELD_IN_BYTES, 4},
	{netflow.NFV9_FIELD_INPUT_SNMP, 4},
	{netflow.NFV9_FIELD_OUTPUT_SNMP, 4},
	{netflow.NFV9_FIELD_FIRST_SWITCHED, 4},
	{netflow.NFV9_FIELD_LAST_SWITCHED, 4},
	{netflow.NFV9_FIELD_L4_SRC_PORT, 2},
	{netflow.NFV9_FIELD_L4_DST_PORT, 2},
	{netflow.NFV9_FIELD_SRC_AS, 4},
	{netflow.NFV9_FIELD_DST_AS, 4},
	{netflow.NFV9_FIELD_PROTOCOL, 1},
	{netflow.NFV9_FIELD_FORWARDING_STATUS, 1},
	{netflow.NFV9_FIELD_FLOW_SAMPLER_ID, 2},
	{netflow.NFV9_FIELD_SRC_MASK, 1},
	{netflow.NFV9_FIELD_DST_MASK, 1},
}

type ipv4Flow struct {
	SrcAddr [4]byte
	DstAddr [4]byte
	IPFlow
}
type ipv6Flow struct {
	SrcAddr [16]byte
	DstAddr [16]byte
	IPFlow
}

func init() {
	ipv4Settings := flowSettings[helpers.ETypeIPv4]
	ipv6Settings := flowSettings[helpers.ETypeIPv6]
	ipv4Settings.FlowLength = binary.Size(ipv4Flow{})
	ipv6Settings.FlowLength = binary.Size(ipv6Flow{})
	ipv4Settings.Template = append([]templateField{
		{netflow.NFV9_FIELD_IPV4_SRC_ADDR, 4},
		{netflow.NFV9_FIELD_IPV4_DST_ADDR, 4},
	}, ipTemplate...)
	ipv6Settings.Template = append([]templateField{
		{netflow.NFV9_FIELD_IPV6_SRC_ADDR, 16},
		{netflow.NFV9_FIELD_IPV6_DST_ADDR, 16},
	}, ipTemplate...)
	// Assuming we have to transmit over IPv6
	ipv4Settings.MaxFlowsPerPacket = 1400 / ipv4Settings.FlowLength
	ipv6Settings.MaxFlowsPerPacket = 1400 / ipv6Settings.FlowLength
}

// getNetflowTemplates returns the payload to define netflow
// templates. UDP payloads are sent on the returned channel. All
// messages should be read to avoid leaking the channel.
func getNetflowTemplates(ctx context.Context, sequenceNumber uint32, sampling int, start, now time.Time) <-chan []byte {
	output := make(chan []byte, 16)
	uptime := uint32(now.Sub(start).Seconds())
	go func() {
		buf := new(bytes.Buffer)
		if err := binary.Write(buf, binary.BigEndian, nfv9Header{
			Version:        9,
			Count:          4,
			SystemUptime:   uptime,
			UnixSeconds:    uint32(now.Unix()),
			SequenceNumber: sequenceNumber,
			SourceID:       0,
		}); err != nil {
			panic(err)
		}
		// IPv4/IPv6 templates
		for _, etype := range []uint16{helpers.ETypeIPv4, helpers.ETypeIPv6} {
			settings := flowSettings[etype]
			if err := binary.Write(buf, binary.BigEndian, flowSetHeader{
				Id:     0,
				Length: uint16(len(settings.Template)*4 + 8),
			}); err != nil {
				panic(err)
			}
			if err := binary.Write(buf, binary.BigEndian, templateRecordHeader{
				TemplateID: settings.TemplateID,
				FieldCount: uint16(len(settings.Template)),
			}); err != nil {
				panic(err)
			}
			if err := binary.Write(buf, binary.BigEndian, settings.Template); err != nil {
				panic(err)
			}
		}
		// Options template
		if err := binary.Write(buf, binary.BigEndian, flowSetHeader{
			Id:     1,
			Length: uint16(26),
		}); err != nil {
			panic(err)
		}
		if err := binary.Write(buf, binary.BigEndian, optionsTemplateRecordHeader{
			TemplateID:   optionsTemplateID,
			ScopeLength:  4,
			OptionLength: 12,
		}); err != nil {
			panic(err)
		}
		if err := binary.Write(buf, binary.BigEndian, []templateField{
			{1, 4}, // system scope
			{netflow.NFV9_FIELD_FLOW_SAMPLER_ID, 2},
			{netflow.NFV9_FIELD_FLOW_SAMPLER_RANDOM_INTERVAL, 4},
			{netflow.NFV9_FIELD_FLOW_SAMPLER_MODE, 1},
		}); err != nil {
			panic(err)
		}
		// Also send the associated data
		if err := binary.Write(buf, binary.BigEndian, flowSetHeader{
			Id:     optionsTemplateID,
			Length: uint16(15),
		}); err != nil {
			panic(err)
		}
		binary.Write(buf, binary.BigEndian, []byte{0xaa, 0xbb, 0xcc, 0xdd}) // system scope
		binary.Write(buf, binary.BigEndian, uint16(1))                      // sampler ID
		binary.Write(buf, binary.BigEndian, uint32(sampling))               // random interval
		binary.Write(buf, binary.BigEndian, uint8(2))                       // mode = random
		select {
		case output <- buf.Bytes():
		case <-ctx.Done():
			return
		}
		defer close(output)
	}()
	return output
}
