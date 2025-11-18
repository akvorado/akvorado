// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"akvorado/common/pb"

	"github.com/google/renameio/v2"
)

// ErrStateVersion is triggered when loading a collection from an incompatible version
var ErrStateVersion = errors.New("collection version mismatch")

// currentStateVersionNumber should be increased each time we change the way we
// encode the collection.
const currentStateVersionNumber = 1

// SaveState save the decoders' state to a file. This is not goroutine-safe.
func (c *Component) SaveState(target string) error {
	state := struct {
		Version  int
		Decoders any
	}{
		Version:  currentStateVersionNumber,
		Decoders: c.decoders,
	}
	data, err := json.Marshal(&state)
	if err != nil {
		return fmt.Errorf("unable to encode decoders' state: %w", err)
	}
	if err := renameio.WriteFile(target, data, 0o666, renameio.WithTempDir(filepath.Dir(target))); err != nil {
		return fmt.Errorf("unable to write state file %q: %w", target, err)
	}
	return nil
}

// RestoreState restores the decoders' state from a file. This is not goroutine-safe.
func (c *Component) RestoreState(source string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("unable to read state file %q: %w", source, err)
	}

	// Check the version.
	var stateVersion struct {
		Version int
	}
	if err := json.Unmarshal(data, &stateVersion); err != nil {
		return err
	}

	if stateVersion.Version != currentStateVersionNumber {
		return ErrStateVersion
	}

	// Decode decoders.
	var stateDecoders struct {
		Decoders map[pb.RawFlow_Decoder]json.RawMessage
	}
	if err := json.Unmarshal(data, &stateDecoders); err != nil {
		return fmt.Errorf("unable to decode decoders' state: %w", err)
	}
	for k, v := range c.decoders {
		decoderJSON, ok := stateDecoders.Decoders[k]
		if !ok {
			continue
		}
		if err := json.Unmarshal(decoderJSON, &v); err != nil {
			return fmt.Errorf("unable to decode decoder' state (%s): %w", k, err)
		}
	}
	return nil
}
