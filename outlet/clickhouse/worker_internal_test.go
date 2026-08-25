// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"errors"
	"testing"
)

// newTestWorker builds a realWorker wired only with what server selection needs
// (no component/metrics), using the provided connect function as the dialer seam.
func newTestWorker(addrs []string, connectFn func(context.Context, *serverConn) error) *realWorker {
	conns := make([]*serverConn, len(addrs))
	for i, a := range addrs {
		conns[i] = &serverConn{address: a}
	}
	return &realWorker{servers: conns, connectFn: connectFn}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// fixedShuffle returns a shuffleFn that always yields the given permutation,
// making the sticky-random pick deterministic in tests.
func fixedShuffle(order ...int) func(int) []int {
	return func(int) []int { return order }
}

// stickyPickN calls pickServerSticky n times and returns the selected addresses.
func stickyPickN(t *testing.T, w *realWorker, n int) []string {
	t.Helper()
	got := make([]string, 0, n)
	for range n {
		sc, err := w.pickServerSticky(context.Background())
		if err != nil {
			t.Fatalf("pickServerSticky() error: %v", err)
		}
		got = append(got, sc.address)
	}
	return got
}

func TestPickServerStickyStaysPinned(t *testing.T) {
	ok := func(context.Context, *serverConn) error { return nil }
	w := newTestWorker([]string{"s0", "s1", "s2"}, ok)
	// The random pick lands on s2 first; the worker must then stay on it.
	w.shuffleFn = fixedShuffle(2, 0, 1)

	got := stickyPickN(t, w, 5)
	want := []string{"s2", "s2", "s2", "s2", "s2"}
	if !equalStrings(got, want) {
		t.Errorf("sticky pin:\n got=%v\nwant=%v", got, want)
	}
}

func TestPickServerStickyInitialFailover(t *testing.T) {
	errBoom := errors.New("boom")
	// s0 fails to connect; the sticky worker should skip it and pin to the next
	// server in shuffle order.
	connectFn := func(_ context.Context, sc *serverConn) error {
		if sc.address == "s0" {
			return errBoom
		}
		return nil
	}
	w := newTestWorker([]string{"s0", "s1", "s2"}, connectFn)
	w.shuffleFn = fixedShuffle(0, 1, 2)

	got := stickyPickN(t, w, 3)
	want := []string{"s1", "s1", "s1"}
	if !equalStrings(got, want) {
		t.Errorf("sticky initial failover:\n got=%v\nwant=%v", got, want)
	}
}

func TestPickServerStickyReconnectBreakRepicks(t *testing.T) {
	down := map[string]bool{}
	connectFn := func(_ context.Context, sc *serverConn) error {
		if down[sc.address] {
			return errors.New("connection broken")
		}
		return nil
	}
	w := newTestWorker([]string{"s0", "s1", "s2"}, connectFn)
	w.shuffleFn = fixedShuffle(0, 1, 2)

	// Pin to s0, then make its connection start failing.
	if got := stickyPickN(t, w, 1); got[0] != "s0" {
		t.Fatalf("expected initial pin to s0, got %s", got[0])
	}
	down["s0"] = true

	// The reuse attempt fails, so the worker drops the pin and re-picks the next
	// healthy server.
	sc, err := w.pickServerSticky(context.Background())
	if err != nil {
		t.Fatalf("pickServerSticky() error: %v", err)
	}
	if sc.address != "s1" {
		t.Errorf("expected re-pick to s1 after s0 connection broke, got %s", sc.address)
	}
}
