// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"net/netip"
	"testing"
	"testing/synctest"
	"time"

	"akvorado/common/helpers"
)

func TestRateLimiterEnforced(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rl := newRateLimiter()
		exporter := netip.MustParseAddr("::ffff:192.0.2.1")
		var rateLimit uint64 = 100

		// Consume the initial burst (100/10 = 10 tokens)
		allowed := 0
		for range 20 {
			if ok, _ := rl.allowOneMessage(exporter, rateLimit); ok {
				allowed++
			}
		}
		if diff := helpers.Diff(allowed, 10); diff != "" {
			t.Fatalf("allow() initial burst (-got, +want):\n%s", diff)
		}

		// After 1 second, we should have 100 more tokens (capped at burst=10)
		time.Sleep(time.Second)
		allowed = 0
		for range 20 {
			if ok, _ := rl.allowOneMessage(exporter, rateLimit); ok {
				allowed++
			}
		}
		if diff := helpers.Diff(allowed, 10); diff != "" {
			t.Fatalf("allow() after 1s (-got, +want):\n%s", diff)
		}
	})
}

func TestRateLimiterDropRate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rl := newRateLimiter()
		exporter := netip.MustParseAddr("::ffff:192.0.2.1")
		var rateLimit uint64 = 100

		// Consume initial burst and some more to create drops
		for range 20 {
			rl.allowOneMessage(exporter, rateLimit)
		}

		// Move to next 200ms tick to update drop rate
		time.Sleep(200 * time.Millisecond)
		_, dropRate := rl.allowOneMessage(exporter, rateLimit)

		// We had 10 allowed, 10 dropped out of 20 total = 50% drop rate
		if diff := helpers.Diff(dropRate, 0.5); diff != "" {
			t.Fatalf("allow() dropRate (-got, +want):\n%s", diff)
		}
	})
}

func TestRateLimiterPerExporter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rl := newRateLimiter()
		exporter1 := netip.MustParseAddr("::ffff:192.0.2.1")
		exporter2 := netip.MustParseAddr("::ffff:192.0.2.2")
		var rateLimit uint64 = 100

		// Exhaust exporter1's burst
		for range 20 {
			rl.allowOneMessage(exporter1, rateLimit)
		}

		// Exporter2 should still have its full burst
		allowed := 0
		for range 20 {
			if ok, _ := rl.allowOneMessage(exporter2, rateLimit); ok {
				allowed++
			}
		}
		if diff := helpers.Diff(allowed, 10); diff != "" {
			t.Fatalf("allow(exporter2) (-got, +want):\n%s", diff)
		}
	})
}

func TestRateLimiterSteadyState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rl := newRateLimiter()
		exporter := netip.MustParseAddr("::ffff:192.0.2.1")
		var rateLimit uint64 = 100

		// Consume initial burst
		for range 20 {
			rl.allowOneMessage(exporter, rateLimit)
		}

		// Send 10 flows per second (below limit) - all should be allowed
		for range 5 {
			time.Sleep(time.Second)
			allowed := 0
			for range 10 {
				if ok, _ := rl.allowOneMessage(exporter, rateLimit); ok {
					allowed++
				}
			}
			if diff := helpers.Diff(allowed, 10); diff != "" {
				t.Fatalf("allow() steady state (-got, +want):\n%s", diff)
			}
		}
	})
}
