// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cache

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

// Save persists the cache to the specified file
func (c *Cache[K, V]) Save(cacheFile string) error {
	tmpFile, err := os.CreateTemp(
		filepath.Dir(cacheFile),
		fmt.Sprintf("%s-*", filepath.Base(cacheFile)))
	if err != nil {
		return fmt.Errorf("unable to create cache file %q: %w", cacheFile, err)
	}
	defer func() {
		tmpFile.Close()           // ignore errors
		os.Remove(tmpFile.Name()) // ignore errors
	}()

	// Write cache
	encoder := gob.NewEncoder(tmpFile)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("unable to encode cache: %w", err)
	}

	// Move cache to new location
	if err := os.Rename(tmpFile.Name(), cacheFile); err != nil {
		return fmt.Errorf("unable to write cache file %q: %w", cacheFile, err)
	}
	return nil
}

// Load loads the cache from the provided location.
func (c *Cache[K, V]) Load(cacheFile string) error {
	f, err := os.Open(cacheFile)
	if err != nil {
		return fmt.Errorf("unable to load cache %q: %w", cacheFile, err)
	}
	defer f.Close()
	decoder := gob.NewDecoder(f)
	if err := decoder.Decode(c); err != nil {
		return fmt.Errorf("unable to decode cache: %w", err)
	}
	return nil
}

// currentVersionNumber should be increased each time we change the way we
// encode the cache.
var currentVersionNumber = 11

// GobEncode encodes the cache
func (c *Cache[K, V]) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	// Encode version
	if err := encoder.Encode(&currentVersionNumber); err != nil {
		return nil, err
	}
	// Encode a representation of K and V. Gob decoding is pretty forgiving, we
	// encode a string representation instead.
	var zeroK K
	var zeroV V
	zero := fmt.Sprintf("%+v\n%+v", zeroK, zeroV)
	if err := encoder.Encode(&zero); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := encoder.Encode(c.items); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode decodes the cache
func (c *Cache[K, V]) GobDecode(data []byte) error {
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)

	// Check version
	version := currentVersionNumber
	if err := decoder.Decode(&version); err != nil {
		return err
	}
	if version != currentVersionNumber {
		return ErrVersion
	}

	// Check correct encoding of K and V
	var zeroK K
	var zeroV V
	var zeroGot string
	zeroExpected := fmt.Sprintf("%+v\n%+v", zeroK, zeroV)
	if err := decoder.Decode(&zeroGot); err != nil {
		return err
	}
	if zeroGot != zeroExpected {
		return ErrVersion
	}
	items := map[K]*item[V]{}
	if err := decoder.Decode(&items); err != nil {
		return err
	}

	c.mu.Lock()
	c.items = items
	c.mu.Unlock()
	return nil
}
