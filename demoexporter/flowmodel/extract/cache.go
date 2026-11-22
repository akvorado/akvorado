// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"os"
)

// SaveToPath writes the result to a file.
func (r *Result) SaveToPath(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create %q: %w", path, err)
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	if err := gob.NewEncoder(out).Encode(r); err != nil {
		return fmt.Errorf("cannot encode %q: %w", path, err)
	}
	if err := out.Flush(); err != nil {
		return fmt.Errorf("cannot write %q: %w", path, err)
	}
	return f.Close()
}

// LoadFromPath reads a result written by SaveToPath.
func LoadFromPath(path string) (*Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r Result
	if err := gob.NewDecoder(bufio.NewReader(f)).Decode(&r); err != nil {
		return nil, fmt.Errorf("cannot decode %q: %w", path, err)
	}
	return &r, nil
}
