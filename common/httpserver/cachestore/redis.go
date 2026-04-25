// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cachestore

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is a Redis-backed Store.
type Redis struct {
	client redis.UniversalClient
}

// NewRedis returns a new Redis-backed Store. The provided client will be
// closed by Redis.Close.
func NewRedis(client redis.UniversalClient) *Redis {
	return &Redis{client: client}
}

// Get implements Store.
func (s *Redis) Get(key string, dst any) error {
	val, err := s.client.Get(context.Background(), key).Bytes()
	if errors.Is(err, redis.Nil) {
		return ErrMiss
	}
	if err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewReader(val)).Decode(dst)
}

// Set implements Store.
func (s *Redis) Set(key string, val any, ttl time.Duration) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(val); err != nil {
		return err
	}
	return s.client.Set(context.Background(), key, buf.Bytes(), ttl).Err()
}

// Delete implements Store.
func (s *Redis) Delete(key string) error {
	return s.client.Del(context.Background(), key).Err()
}

// Close closes the underlying Redis client.
func (s *Redis) Close() error {
	return s.client.Close()
}
