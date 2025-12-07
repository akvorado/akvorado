// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package embed

import (
	_ "embed"
	"io"
	"testing"

	"akvorado/common/helpers"
)

func TestData(t *testing.T) {
	f, err := Data().Open("orchestrator/clickhouse/data/protocols.csv")
	if err != nil {
		t.Fatalf("Open() error:\n%+v", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Fatalf("Close() error:\n%+v", err)
		}
	}()
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

func BenchmarkData(b *testing.B) {
	const amount = 64 * 1024 * 1024
	got := make([]byte, amount)
	b.Run("compressed", func(b *testing.B) {
		for b.Loop() {
			f, _ := Data().Open("orchestrator/clickhouse/data/tcp.csv")
			_, _ = io.ReadFull(f, got)
			f.Close()
		}
	})
	b.Run("uncompressed", func(b *testing.B) {
		for b.Loop() {
			copy(got, embeddedZip)
		}
	})

}
