// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package embed_test

import (
	"io"
	"testing"

	"akvorado/common/embed"
	"akvorado/common/helpers"
)

func TestData(t *testing.T) {
	f, err := embed.Data().Open("orchestrator/clickhouse/data/protocols.csv")
	if err != nil {
		t.Fatalf("Open() error:\n%+v", err)
	}
	expected := "proto,name,description"
	got := make([]byte, len(expected))
	_, err = io.ReadFull(f, got)
	if err != nil {
		t.Fatalf("ReadFull() error:\n%+v", err)
	}
	if diff := helpers.Diff(string(got), expected); diff != "" {
		t.Fatalf("ReadFull() (-got, +want):\n%s", diff)
	}
}
