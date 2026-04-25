// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cachestore_test

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/httpserver/cachestore"
)

func TestMemoryGetSet(t *testing.T) {
	store := cachestore.NewMemory(time.Minute)
	defer store.Close()

	if err := store.Set("k", "v", time.Minute); err != nil {
		t.Fatalf("Set() error:\n%+v", err)
	}
	var got string
	if err := store.Get("k", &got); err != nil {
		t.Fatalf("Get() error:\n%+v", err)
	}
	if diff := helpers.Diff(got, "v"); diff != "" {
		t.Fatalf("Get() (-got, +want):\n%s", diff)
	}

	if err := store.Delete("k"); err != nil {
		t.Fatalf("Delete() error:\n%+v", err)
	}
	if err := store.Get("k", &got); !errors.Is(err, cachestore.ErrMiss) {
		t.Fatalf("Get() after Delete: got %+v, want ErrMiss", err)
	}
}

func TestMemoryExpiration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Sweeper interval > TTL so we exercise lazy expiration first,
		// then sweeper-driven cleanup.
		store := cachestore.NewMemory(time.Minute)
		defer store.Close()

		if err := store.Set("k", "v", 30*time.Second); err != nil {
			t.Fatalf("Set() error:\n%+v", err)
		}
		var got string
		if err := store.Get("k", &got); err != nil {
			t.Fatalf("Get() before expiry error:\n%+v", err)
		}

		// Lazy expiration: past the TTL, Get returns ErrMiss even before
		// the sweeper has had a chance to run.
		time.Sleep(31 * time.Second)
		if err := store.Get("k", &got); !errors.Is(err, cachestore.ErrMiss) {
			t.Fatalf("Get() after expiry: got %+v, want ErrMiss", err)
		}

		// Refill, then wait long enough for the sweeper to tick at least
		// once after expiry.
		if err := store.Set("k2", "v2", 30*time.Second); err != nil {
			t.Fatalf("Set() error:\n%+v", err)
		}
		time.Sleep(61 * time.Second)
		synctest.Wait()
		if err := store.Get("k2", &got); !errors.Is(err, cachestore.ErrMiss) {
			t.Fatalf("Get() after sweep: got %+v, want ErrMiss", err)
		}
	})
}

func TestMemoryClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := cachestore.NewMemory(time.Minute)
		// Calling Close twice must not panic and must not deadlock.
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error:\n%+v", err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("Close() second call error:\n%+v", err)
		}
		// After Close, the sweeper goroutine has stopped, so no more
		// goroutines.
		synctest.Wait()
	})
}
