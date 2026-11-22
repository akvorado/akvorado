// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"os"
	"path/filepath"
	"testing"

	"akvorado/common/helpers"
	"akvorado/demoexporter/flowmodel"
)

func TestCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache")
	example := flowmodel.Example{Weight: 3}
	example.Tokens[flowmodel.FieldRemotePrefix] = 2
	expected := Result{
		Metadata: flowmodel.MockMetadata(),
		Examples: []flowmodel.Example{example},
		Flows:    10,
	}
	if err := expected.SaveToPath(path); err != nil {
		t.Fatalf("SaveToPath() error:\n%+v", err)
	}
	got, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath() error:\n%+v", err)
	}
	if diff := helpers.Diff(*got, expected); diff != "" {
		t.Fatalf("LoadFromPath() (-got, +want):\n%s", diff)
	}

	if err := os.WriteFile(path, []byte("garbage"), 0o644); err != nil {
		t.Fatalf("WriteFile() error:\n%+v", err)
	}
	if _, err := LoadFromPath(path); err == nil {
		t.Fatal("LoadFromPath() should fail on a corrupted file")
	}
}
