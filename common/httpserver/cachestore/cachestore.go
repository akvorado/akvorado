// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package cachestore defines the cache backend interface used by the
// HTTP cache middleware along with in-memory and Redis implementations.
package cachestore

import (
	"errors"
	"time"
)

// ErrMiss signals a cache miss.
var ErrMiss = errors.New("cache miss")

// Store is a generic cache backend.
type Store interface {
	// Get retrieves a previously cached value, decoding it into dst. It
	// returns ErrMiss when the key is not present or has expired.
	Get(key string, dst any) error
	// Set stores val under key with the given TTL.
	Set(key string, val any, ttl time.Duration) error
	// Delete removes the entry for key.
	Delete(key string) error
	// Close releases any resources held by the store.
	Close() error
}
