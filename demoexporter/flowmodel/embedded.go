// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flowmodel

import (
	"bytes"
	_ "embed" // for default model
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// The default model is committed to the repository and embedded in the binary.
// It is built with `akvorado demo-exporter train`.
//
//go:embed data/default.model
var defaultModel []byte

// LoadFromPath loads a model from a file. An empty path loads the embedded
// default model.
func LoadFromPath(path string) (*Model, error) {
	if path == "" {
		return Load(bytes.NewReader(defaultModel))
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open model: %w", err)
	}
	defer f.Close()
	model, err := Load(f)
	if err != nil {
		return nil, fmt.Errorf("cannot load model %q: %w", path, err)
	}
	return model, nil
}

// SaveToPath writes the model to a file. The file is replaced only when the
// model is completely written.
func (m *Model) SaveToPath(path string) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".model-*")
	if err != nil {
		return fmt.Errorf("cannot create model: %w", err)
	}
	defer os.Remove(f.Name())
	err = errors.Join(m.Save(f), f.Chmod(0o644), f.Close())
	if err != nil {
		return fmt.Errorf("cannot write model %q: %w", path, err)
	}
	return os.Rename(f.Name(), path)
}
