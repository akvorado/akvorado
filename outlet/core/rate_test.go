// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"net/netip"
	"testing"

	"akvorado/common/helpers"
)

func TestRateLimiterEnforced(t *testing.T) {
	rl := newRateLimiter()
	exporter := netip.MustParseAddr("::ffff:192.0.2.1")
	var rateLimit uint64 = 100

	// Only the first 100 flows of a second are accepted
	allowed := 0
	for range 150 {
		if ok, _ := rl.allowOneMessage(exporter, rateLimit, 1000); ok {
			allowed++
		}
	}
	if diff := helpers.Diff(allowed, 100); diff != "" {
		t.Fatalf("allow() during first second (-got, +want):\n%s", diff)
	}

	// The next second starts with a fresh budget
	allowed = 0
	for range 150 {
		if ok, _ := rl.allowOneMessage(exporter, rateLimit, 1001); ok {
			allowed++
		}
	}
	if diff := helpers.Diff(allowed, 100); diff != "" {
		t.Fatalf("allow() during second second (-got, +want):\n%s", diff)
	}
}

func TestRateLimiterSamplingRateFactor(t *testing.T) {
	rl := newRateLimiter()
	exporter := netip.MustParseAddr("::ffff:192.0.2.1")
	var rateLimit uint64 = 100

	// 150 flows received each second, 100 accepted. The factor of a second is
	// only known once the next one starts and it does not accumulate over the
	// seconds.
	for second := range uint64(5) {
		expected := float64(1.5)
		if second == 0 {
			expected = 1
		}
		for range 150 {
			_, factor := rl.allowOneMessage(exporter, rateLimit, 1000+second)
			if diff := helpers.Diff(factor, expected); diff != "" {
				t.Fatalf("allow() factor at second %d (-got, +want):\n%s", second, diff)
			}
		}
	}
}

func TestRateLimiterLateFlow(t *testing.T) {
	rl := newRateLimiter()
	exporter := netip.MustParseAddr("::ffff:192.0.2.1")
	var rateLimit uint64 = 100

	// Use a part of the budget of a second
	for range 60 {
		rl.allowOneMessage(exporter, rateLimit, 1001)
	}

	// Flows from an earlier second share this budget instead of starting a new
	// one: only the 40 remaining ones are accepted.
	allowed := 0
	for range 50 {
		if ok, _ := rl.allowOneMessage(exporter, rateLimit, 1000); ok {
			allowed++
		}
	}
	if diff := helpers.Diff(allowed, 40); diff != "" {
		t.Fatalf("allow() late flows (-got, +want):\n%s", diff)
	}
}

func TestRateLimiterPerExporter(t *testing.T) {
	rl := newRateLimiter()
	exporter1 := netip.MustParseAddr("::ffff:192.0.2.1")
	exporter2 := netip.MustParseAddr("::ffff:192.0.2.2")
	var rateLimit uint64 = 100

	// Use up exporter1's budget
	for range 150 {
		rl.allowOneMessage(exporter1, rateLimit, 1000)
	}

	// Exporter2 should still have its full budget
	allowed := 0
	for range 100 {
		if ok, _ := rl.allowOneMessage(exporter2, rateLimit, 1000); ok {
			allowed++
		}
	}
	if diff := helpers.Diff(allowed, 100); diff != "" {
		t.Fatalf("allow(exporter2) (-got, +want):\n%s", diff)
	}
}

func TestRateLimiterBelowLimit(t *testing.T) {
	rl := newRateLimiter()
	exporter := netip.MustParseAddr("::ffff:192.0.2.1")
	var rateLimit uint64 = 100

	// Send 10 flows per second, below the limit: nothing is dropped and the
	// sampling rate is left alone.
	for second := range uint64(5) {
		for range 10 {
			ok, factor := rl.allowOneMessage(exporter, rateLimit, 1000+second)
			if !ok {
				t.Fatalf("allow() dropped a flow at second %d", second)
			}
			if diff := helpers.Diff(factor, float64(1)); diff != "" {
				t.Fatalf("allow() factor at second %d (-got, +want):\n%s", second, diff)
			}
		}
	}
}
