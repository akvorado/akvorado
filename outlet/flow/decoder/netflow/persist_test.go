// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"encoding/json"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/netsampler/goflow2/v2/decoders/netflow"
)

func TestMarshalUnmarshalTemplates(t *testing.T) {
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	nfdecoder := New(r, decoder.Dependencies{Schema: sch})
	collection := &nfdecoder.(*Decoder).collection
	exporter := collection.Get("::ffff:192.168.1.1")
	exporter.SetSamplingRate(10, 300, 10, 2048)
	exporter.SetSamplingRate(9, 301, 11, 4096)
	exporter.AddTemplate(10, 300, 300, netflow.TemplateRecord{
		TemplateId: 300,
		FieldCount: 2,
		Fields: []netflow.Field{
			{
				Type:   netflow.IPFIX_FIELD_applicationName,
				Length: 10,
			}, {
				Type:   netflow.IPFIX_FIELD_VRFname,
				Length: 25,
			},
		},
	})
	exporter.AddTemplate(10, 300, 301, netflow.IPFIXOptionsTemplateRecord{
		TemplateId:      301,
		FieldCount:      2,
		ScopeFieldCount: 0,
		Options: []netflow.Field{
			{
				Type:   netflow.IPFIX_FIELD_samplerRandomInterval,
				Length: 4,
			}, {
				Type:   netflow.IPFIX_FIELD_samplerMode,
				Length: 4,
			},
		},
	})
	exporter.AddTemplate(9, 301, 300, netflow.NFv9OptionsTemplateRecord{
		TemplateId:   300,
		OptionLength: 2,
		Options: []netflow.Field{
			{
				Type:   netflow.NFV9_FIELD_FLOW_ACTIVE_TIMEOUT,
				Length: 4,
			}, {
				Type:   netflow.NFV9_FIELD_FORWARDING_STATUS,
				Length: 2,
			},
		},
	})

	jsonBytes, err := json.Marshal(&nfdecoder)
	if err != nil {
		t.Fatalf("json.Marshal() error:\n%+v", err)
	}
	nfdecoder2 := New(r, decoder.Dependencies{Schema: sch})
	if err := json.Unmarshal(jsonBytes, &nfdecoder2); err != nil {
		t.Fatalf("json.Unmarshal() error:\n%+v", err)
	}

	collection1 := &nfdecoder.(*Decoder).collection.Collection
	collection2 := &nfdecoder2.(*Decoder).collection.Collection
	if diff := helpers.Diff(collection1, collection2,
		cmpopts.IgnoreUnexported(templatesAndOptions{})); diff != "" {
		t.Fatalf("json.Marshal()/json.Unmarshal() (-got, +want):\n%s", diff)
	}
}
