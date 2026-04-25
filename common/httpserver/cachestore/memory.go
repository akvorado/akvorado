// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cachestore

import (
	"bytes"
	"encoding/gob"
	"sync"
	"time"
)

// Memory is an in-memory Store. Entries expire lazily on access and via a
// background sweeper goroutine that runs until Close is called.
type Memory struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	done    chan struct{}
}

type memoryEntry struct {
	value   []byte
	expires time.Time
}

// NewMemory returns a new in-memory Store. The sweeper runs at
// cleanupInterval.
func NewMemory(cleanupInterval time.Duration) *Memory {
	s := &Memory{
		entries: map[string]memoryEntry{},
		done:    make(chan struct{}),
	}
	go s.sweep(cleanupInterval)
	return s
}

func (s *Memory) sweep(every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			now := time.Now()
			s.mu.Lock()
			for k, e := range s.entries {
				if now.After(e.expires) {
					delete(s.entries, k)
				}
			}
			s.mu.Unlock()
		}
	}
}

// Get implements Store.
func (s *Memory) Get(key string, dst any) error {
	s.mu.Lock()
	e, ok := s.entries[key]
	s.mu.Unlock()
	if !ok || time.Now().After(e.expires) {
		return ErrMiss
	}
	return gob.NewDecoder(bytes.NewReader(e.value)).Decode(dst)
}

// Set implements Store.
func (s *Memory) Set(key string, val any, ttl time.Duration) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(val); err != nil {
		return err
	}
	s.mu.Lock()
	s.entries[key] = memoryEntry{value: buf.Bytes(), expires: time.Now().Add(ttl)}
	s.mu.Unlock()
	return nil
}

// Delete implements Store.
func (s *Memory) Delete(key string) error {
	s.mu.Lock()
	delete(s.entries, key)
	s.mu.Unlock()
	return nil
}

// Close stops the sweeper goroutine.
func (s *Memory) Close() error {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	return nil
}
