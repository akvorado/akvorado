// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flowmodel

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// The model file starts with a small header, followed by the metadata
// as JSON and the weight tensors as little-endian float32.
var modelMagic = []byte("AKFM")

const modelVersion = 1

// Save writes the model to a writer.
func (m *Model) Save(w io.Writer) error {
	out := bufio.NewWriter(w)

	meta, err := json.Marshal(m.Metadata)
	if err != nil {
		return fmt.Errorf("cannot encode metadata: %w", err)
	}
	out.Write(modelMagic)
	out.WriteByte(modelVersion)
	binary.Write(out, binary.LittleEndian, uint32(len(meta)))
	out.Write(meta)
	for _, tensor := range m.w.all() {
		binary.Write(out, binary.LittleEndian, tensor)
	}
	if err := out.Flush(); err != nil {
		return fmt.Errorf("cannot write model: %w", err)
	}
	return nil
}

// Load reads a model from a reader.
func Load(r io.Reader) (*Model, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("cannot read model: %w", err)
	}
	const headerLen = 4 + 1 + 4
	if len(data) < headerLen {
		return nil, errors.New("model file too short")
	}
	if string(data[:4]) != string(modelMagic) {
		return nil, errors.New("not a flow model file")
	}
	if data[4] != modelVersion {
		return nil, fmt.Errorf("unsupported model file version %d", data[4])
	}
	metaLen := int(binary.LittleEndian.Uint32(data[5:headerLen]))
	if headerLen+metaLen > len(data) {
		return nil, errors.New("corrupted model file: truncated metadata")
	}
	m := Model{}
	if err := json.Unmarshal(data[headerLen:headerLen+metaLen], &m.Metadata); err != nil {
		return nil, fmt.Errorf("cannot decode model metadata: %w", err)
	}

	m.w = newWeights(&m.Metadata, nil)
	body := bytes.NewReader(data[headerLen+metaLen:])
	for _, tensor := range m.w.all() {
		if err := binary.Read(body, binary.LittleEndian, tensor); err != nil {
			return nil, errors.New("corrupted model file: truncated tensor")
		}
	}
	if body.Len() != 0 {
		return nil, errors.New("corrupted model file: trailing data")
	}
	return &m, nil
}
