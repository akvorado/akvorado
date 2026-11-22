// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flowmodel

import (
	"bytes"
	"testing"

	"akvorado/common/helpers"
)

func TestSaveLoad(t *testing.T) {
	model := NewMock(t)
	var buf bytes.Buffer
	if err := model.Save(&buf); err != nil {
		t.Fatalf("Save() error:\n%+v", err)
	}
	if buf.Len() > 1<<20 {
		t.Fatalf("Save() produced %d bytes, expected less than 1MB", buf.Len())
	}

	got, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Load() error:\n%+v", err)
	}
	if diff := helpers.Diff(got.Metadata, model.Metadata); diff != "" {
		t.Fatalf("Load() metadata (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(got.w.all(), model.w.all()); diff != "" {
		t.Fatalf("Load() weights (-got, +want):\n%s", diff)
	}
}

func TestLoadErrors(t *testing.T) {
	model := NewMock(t)
	var buf bytes.Buffer
	if err := model.Save(&buf); err != nil {
		t.Fatalf("Save() error:\n%+v", err)
	}
	data := buf.Bytes()

	cases := []struct {
		Description string
		Mutate      func([]byte) []byte
	}{
		{"empty file", func([]byte) []byte { return nil }},
		{"bad magic", func(d []byte) []byte {
			d = bytes.Clone(d)
			d[0] = 'X'
			return d
		}},
		{"bad version", func(d []byte) []byte {
			d = bytes.Clone(d)
			d[4] = 42
			return d
		}},
		{"truncated", func(d []byte) []byte { return d[:len(d)-10] }},
		{"trailing data", func(d []byte) []byte { return append(bytes.Clone(d), 0) }},
	}
	for _, tc := range cases {
		if _, err := Load(bytes.NewReader(tc.Mutate(data))); err == nil {
			t.Errorf("Load() did not error on %s", tc.Description)
		}
	}
}
