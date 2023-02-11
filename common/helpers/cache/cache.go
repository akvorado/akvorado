// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package cache implements a cache with an optional TTL. Each operation should
// provide the current time. Items are expired on demand. Expiration can be done
// on last access or last update. Due to an implementation detail, it relies on
// wall time.
package cache

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrVersion is triggered when loading a cache from an incompatible version
var ErrVersion = errors.New("cache version mismatch")

// Cache is a thread-safe in-memory key/value store
type Cache[K comparable, V any] struct {
	items map[K]*item[V]
	mu    sync.RWMutex
}

// item is a cache item, including last access and last update
type item[V any] struct {
	Object       V
	LastAccessed int64
	LastUpdated  int64
}

// New creates a new instance of the cache with the specified duration.
func New[K comparable, V any]() *Cache[K, V] {
	return &Cache[K, V]{
		items: make(map[K]*item[V]),
	}
}

func (c *Cache[K, V]) zero() V {
	var v V
	return v
}

// Put adds a new object in the cache.
func (c *Cache[K, V]) Put(now time.Time, key K, object V) {
	n := now.Unix()
	item := item[V]{
		Object:       object,
		LastAccessed: n,
		LastUpdated:  n,
	}
	c.mu.Lock()
	c.items[key] = &item
	c.mu.Unlock()
}

// Get retrieves an object from the cache. If now is uninitialized, time of last
// access is not updated.
func (c *Cache[K, V]) Get(now time.Time, key K) (V, bool) {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return c.zero(), false
	}
	if !now.IsZero() {
		n := now.Unix()
		atomic.StoreInt64(&item.LastAccessed, n)
	}
	return item.Object, true
}

// Items retrieve all the key/value in the cache.
func (c *Cache[K, V]) Items() map[K]V {
	result := map[K]V{}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for k, v := range c.items {
		result[k] = v.Object
	}
	return result
}

// ItemsLastUpdatedBefore returns the items whose last update is before the
// provided time.
func (c *Cache[K, V]) ItemsLastUpdatedBefore(before time.Time) map[K]V {
	result := map[K]V{}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for k, v := range c.items {
		if v.LastUpdated < before.Unix() {
			result[k] = v.Object
		}
	}
	return result
}

// DeleteLastAccessedBefore expires items whose last access is before
// the provided time.
func (c *Cache[K, V]) DeleteLastAccessedBefore(before time.Time) int {
	count := 0
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range c.items {
		last := atomic.LoadInt64(&v.LastAccessed)
		if last < before.Unix() {
			delete(c.items, k)
			count++
		}
	}
	return count
}

// Size returns the size of the cache
func (c *Cache[K, V]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}
