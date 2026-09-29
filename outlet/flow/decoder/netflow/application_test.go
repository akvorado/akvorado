// SPDX-FileCopyrightText: 2026 ioplane
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"encoding/binary"
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"
)

func TestFormatApplicationID(t *testing.T) {
	cases := []struct {
		expected string
		id       []byte
	}{
		{"13:453", []byte{13, 0, 0x01, 0xc5}},
		{"3:80", []byte{3, 0, 0, 80}},
		{"3:80", []byte{3, 0, 80}},
		{"20:9:10000", []byte{20, 0, 0, 0, 9, 0x27, 0x10}},
		{"0d", []byte{13}},
	}
	for _, tc := range cases {
		if got := formatApplicationID(tc.id); got != tc.expected {
			t.Errorf("formatApplicationID(%x) = %q, expected %q", tc.id, got, tc.expected)
		}
	}
}

func TestApplicationKeyMarshalText(t *testing.T) {
	key := applicationKey{id: string([]byte{13, 0, 1, 0xc5}), version: 9}
	text, err := key.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() error:\n%+v", err)
	}
	var got applicationKey
	if err := got.UnmarshalText(text); err != nil {
		t.Fatalf("UnmarshalText(%q) error:\n%+v", text, err)
	}
	if got != key {
		t.Errorf("UnmarshalText(%q) = %+v, expected %+v", text, got, key)
	}
}

// nbarField is a template field with its value.
type nbarField struct {
	value []byte
	id    uint16
	pen   uint32
}

func nbarString(s string, size int) []byte {
	b := make([]byte, size)
	copy(b, s)
	return b
}

func nbarAppID(engine byte, selector uint32) []byte {
	return []byte{engine, byte(selector >> 16), byte(selector >> 8), byte(selector)}
}

// nbarMessage builds a NetFlow v9 packet or an IPFIX message with one set.
func nbarMessage(version uint16, domain uint32, setID uint16, content []byte) []byte {
	be16 := binary.BigEndian.AppendUint16
	be32 := binary.BigEndian.AppendUint32
	set := append(be16(be16(nil, setID), uint16(4+len(content))), content...)
	if version == 9 {
		msg := be16(be16(nil, 9), 1)
		msg = be32(be32(be32(be32(msg, 0), 1_700_000_000), 1), domain)
		return append(msg, set...)
	}
	msg := be16(be16(nil, 10), uint16(16+len(set)))
	msg = be32(be32(be32(msg, 1_700_000_000), 1), domain)
	return append(msg, set...)
}

// nbarFieldSpecifiers encodes field specifiers. For IPFIX, an enterprise
// number sets the enterprise bit.
func nbarFieldSpecifiers(version uint16, fields []nbarField) []byte {
	var b []byte
	for _, f := range fields {
		id := f.id
		if version == 10 && f.pen != 0 {
			id |= 0x8000
		}
		b = binary.BigEndian.AppendUint16(b, id)
		b = binary.BigEndian.AppendUint16(b, uint16(len(f.value)))
		if version == 10 && f.pen != 0 {
			b = binary.BigEndian.AppendUint32(b, f.pen)
		}
	}
	return b
}

// nbarOptions builds an options template and its data record, with a system
// scope like Cisco IOS XE.
func nbarOptions(
	version uint16,
	domain uint32,
	templateID uint16,
	records ...[]nbarField,
) [][]byte {
	be16 := binary.BigEndian.AppendUint16
	scope := nbarField{[]byte{192, 0, 2, 1}, 1, 0}
	specs := nbarFieldSpecifiers(version, records[0])
	var template []byte
	if version == 9 {
		template = be16(be16(be16(nil, templateID), 4), uint16(len(specs)))
		template = append(be16(be16(template, 1), 4), specs...)
	} else {
		template = be16(be16(be16(nil, templateID), uint16(1+len(records[0]))), 1)
		// IPFIX scope: exporter IPv4 address.
		template = append(be16(be16(template, 130), 4), specs...)
	}
	var data []byte
	for _, record := range records {
		data = append(data, scope.value...)
		for _, f := range record {
			data = append(data, f.value...)
		}
	}
	setID := uint16(1)
	if version == 10 {
		setID = 3
	}
	return [][]byte{
		nbarMessage(version, domain, setID, template),
		nbarMessage(version, domain, templateID, data),
	}
}

// nbarFlows builds a data template with addresses and applicationId, and
// one record per application ID.
func nbarFlows(version uint16, domain uint32, ids ...[]byte) [][]byte {
	be16 := binary.BigEndian.AppendUint16
	template := be16(be16(nil, 263), 3)
	template = be16(be16(be16(be16(be16(be16(template, 8), 4), 12), 4), 95), 4)
	var data []byte
	for _, id := range ids {
		data = append(data, 192, 0, 2, 10, 198, 51, 100, 10)
		data = append(data, id...)
	}
	setID := uint16(0)
	if version == 10 {
		setID = 2
	}
	return [][]byte{
		nbarMessage(version, domain, setID, template),
		nbarMessage(version, domain, 263, data),
	}
}

func decodeNBAR(t *testing.T, packets ...[]byte) []map[schema.ColumnKey]any {
	t.Helper()
	_, nfdecoder, bf, got, finalize := setup(t, false)
	for _, packet := range packets {
		_, err := nfdecoder.Decode(decoder.RawFlow{
			Payload:      packet,
			Source:       netip.MustParseAddr("::ffff:127.0.0.1"),
			TimeReceived: time.Unix(1_700_000_000, 0),
		}, decoder.Options{TimestampSource: pb.RawFlow_TS_INPUT}, bf, finalize)
		if err != nil {
			t.Fatalf("Decode() error:\n%+v", err)
		}
	}
	result := []map[schema.ColumnKey]any{}
	for _, flow := range *got {
		columns := map[schema.ColumnKey]any{}
		for _, column := range applicationColumns {
			if value, ok := flow.OtherColumns[column]; ok {
				columns[column] = value
			}
		}
		result = append(result, columns)
	}
	return result
}

func TestDecodeNBARApplicationNetFlowV9(t *testing.T) {
	ssl, http, unknown := nbarAppID(13, 453), nbarAppID(3, 80), nbarAppID(13, 9999)
	// Same layout as Cisco IOS XE 17.18 option application-table (template
	// 258) and option application-attributes (template 259), exported with
	// source ID 6 while flows use another source ID.
	names := nbarOptions(
		9,
		6,
		258,
		[]nbarField{
			{ssl, 95, 0},
			{nbarString("ssl", 24), 96, 0},
			{nbarString("Secure Socket Layer", 55), 94, 0},
		},
		[]nbarField{
			{http, 95, 0},
			{nbarString("http", 24), 96, 0},
			{nbarString("World Wide Web traffic", 55), 94, 0},
		},
	)
	attributes := nbarOptions(9, 6, 259, []nbarField{
		{ssl, 95, 0},
		{nbarString("browsing", 32), 45000, 0},
		{nbarString("other", 32), 45001, 0},
		{nbarString("other", 32), 45002, 0},
		{nbarString("transactional-data", 32), 45011, 0},
		{nbarString("business-relevant", 32), 45012, 0},
		{nbarString("no", 10), 288, 0},
		{nbarString("no", 10), 289, 0},
		{nbarString("yes", 10), 290, 0},
		{nbarString("general-browsing", 32), 44999, 0},
		{nbarString("web", 32), 44998, 0},
	})
	packets := append(append(names, attributes...), nbarFlows(9, 768, ssl, http, unknown)...)

	got := decodeNBAR(t, packets...)
	expected := []map[schema.ColumnKey]any{
		{
			schema.ColumnApplication:                  "ssl",
			schema.ColumnApplicationCategory:          "browsing",
			schema.ColumnApplicationSubCategory:       "other",
			schema.ColumnApplicationGroup:             "other",
			schema.ColumnApplicationTrafficClass:      "transactional-data",
			schema.ColumnApplicationBusinessRelevance: "business-relevant",
			schema.ColumnApplicationFamily:            "web",
			schema.ColumnApplicationSet:               "general-browsing",
			schema.ColumnApplicationP2P:               "no",
			schema.ColumnApplicationTunnel:            "no",
			schema.ColumnApplicationEncrypted:         "yes",
		},
		{schema.ColumnApplication: "http"},
		{schema.ColumnApplication: "13:9999"},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}

func TestDecodeNBARApplicationIPFIX(t *testing.T) {
	webex := nbarAppID(13, 10000)
	names := nbarOptions(10, 0, 258, []nbarField{
		{webex, 95, 0},
		{nbarString("webex-meeting", 24), 96, 0},
	})
	attributes := nbarOptions(10, 0, 259, []nbarField{
		{webex, 95, 0},
		{nbarString("voice-and-video", 32), 12232, ciscoPEN},
		{nbarString("multimedia", 32), 12243, ciscoPEN},
		{nbarString("collaboration", 32), 374, 0},
	})
	packets := append(append(names, attributes...), nbarFlows(10, 0, webex)...)

	got := decodeNBAR(t, packets...)
	expected := []map[schema.ColumnKey]any{{
		schema.ColumnApplication:             "webex-meeting",
		schema.ColumnApplicationCategory:     "voice-and-video",
		schema.ColumnApplicationTrafficClass: "multimedia",
		schema.ColumnApplicationGroup:        "collaboration",
	}}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}

func TestDecodeNBARApplicationBeforeOptions(t *testing.T) {
	ssl := nbarAppID(13, 453)
	got := decodeNBAR(t, nbarFlows(9, 768, ssl)...)
	expected := []map[schema.ColumnKey]any{{schema.ColumnApplication: "13:453"}}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}

func TestApplicationsPersisted(t *testing.T) {
	tao := &templatesAndOptions{}
	var update application
	update[applicationName] = "ssl"
	tao.UpdateApplication(9, nbarAppID(13, 453), &update)
	raw, err := json.Marshal(tao)
	if err != nil {
		t.Fatalf("json.Marshal() error:\n%+v", err)
	}
	var restored templatesAndOptions
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatalf("json.Unmarshal(%s) error:\n%+v", raw, err)
	}
	if app, ok := restored.GetApplication(
		9,
		nbarAppID(13, 453),
	); !ok ||
		app[applicationName] != "ssl" {
		t.Errorf("GetApplication() after restore = %v, %v", app, ok)
	}
}
