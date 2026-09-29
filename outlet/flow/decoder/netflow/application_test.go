// SPDX-FileCopyrightText: 2026 ioplane
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/pb"
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

// decodeApplications decodes captures from a Cisco C8000V running IOS XE
// 17.18.03a with NBAR2 protocol pack 78.0 (options with source ID 6, flows
// with another source ID) and counts the flows for each set of application
// columns.
func decodeApplications(t *testing.T, format string) map[string]int {
	t.Helper()
	_, nfdecoder, bf, got, finalize := setup(t, false)
	pcap := filepath.Join("testdata", fmt.Sprintf("iosxe-%s-nbar2.pcap", format))
	for data := range helpers.ReadManyPcapL4(t, pcap) {
		_, err := nfdecoder.Decode(decoder.RawFlow{
			Payload: data,
			Source:  netip.MustParseAddr("::ffff:127.0.0.1"),
		}, decoder.Options{TimestampSource: pb.RawFlow_TS_INPUT}, bf, finalize)
		if err != nil {
			t.Fatalf("Decode() error:\n%+v", err)
		}
	}
	result := map[string]int{}
	for _, flow := range *got {
		values := []string{}
		for _, column := range applicationColumns {
			if value, ok := flow.OtherColumns[column]; ok {
				values = append(values, fmt.Sprintf("%s=%s", column, value))
			}
		}
		result[strings.Join(values, " ")]++
	}
	return result
}

func TestDecodeApplications(t *testing.T) {
	expected := map[string]int{
		"Application=ssl ApplicationCategory=browsing " +
			"ApplicationSubCategory=enterprise-transactional-apps ApplicationGroup=other " +
			"ApplicationTrafficClass=transactional-data ApplicationBusinessRelevance=default " +
			"ApplicationFamily=encrypted ApplicationSet=general-browsing " +
			"ApplicationP2P=no ApplicationTunnel=yes ApplicationEncrypted=yes": 11,
		// Not in the captured part of the application table.
		"Application=13:1297": 6,
		// Attributes captured, but not the name.
		"Application=13:1 ApplicationCategory=other ApplicationSubCategory=other " +
			"ApplicationGroup=other ApplicationTrafficClass=bulk-data " +
			"ApplicationBusinessRelevance=default ApplicationFamily=network-service " +
			"ApplicationSet=general-misc ApplicationP2P=no ApplicationTunnel=no " +
			"ApplicationEncrypted=no": 1,
	}
	for _, format := range []string{"v9", "ipfix"} {
		t.Run(format, func(t *testing.T) {
			if diff := helpers.Diff(decodeApplications(t, format), expected); diff != "" {
				t.Fatalf("Decode() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestApplicationsPersisted(t *testing.T) {
	tao := &templatesAndOptions{}
	var update application
	update[applicationName] = "ssl"
	tao.UpdateApplication(9, []byte{13, 0, 0x01, 0xc5}, &update)
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
		[]byte{13, 0, 0x01, 0xc5},
	); !ok ||
		app[applicationName] != "ssl" {
		t.Errorf("GetApplication() after restore = %v, %v", app, ok)
	}
}
