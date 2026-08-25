// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"errors"
	"testing"
	"time"

	"akvorado/common/reporter"
	"akvorado/common/schema"
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

// pickN calls pickServer n times and returns the sequence of selected addresses.
func pickN(t *testing.T, w *realWorker, n int) []string {
	t.Helper()
	got := make([]string, 0, n)
	for range n {
		sc, err := w.pickServer(context.Background())
		if err != nil {
			t.Fatalf("pickServer() error: %v", err)
		}
		got = append(got, sc.address)
	}
	return got
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

func TestPickServerRoundRobin(t *testing.T) {
	ok := func(context.Context, *serverConn) error { return nil }
	w := newTestWorker([]string{"s0", "s1", "s2"}, ok)

	got := pickN(t, w, 7)
	want := []string{"s0", "s1", "s2", "s0", "s1", "s2", "s0"}
	if !equalStrings(got, want) {
		t.Errorf("round-robin order:\n got=%v\nwant=%v", got, want)
	}
}

func TestPickServerStaggeredStart(t *testing.T) {
	ok := func(context.Context, *serverConn) error { return nil }
	// A worker whose cursor starts at index 1 (staggered by worker id) should
	// begin on s1, not s0.
	w := newTestWorker([]string{"s0", "s1", "s2"}, ok)
	w.next = 1

	got := pickN(t, w, 3)
	want := []string{"s1", "s2", "s0"}
	if !equalStrings(got, want) {
		t.Errorf("staggered start:\n got=%v\nwant=%v", got, want)
	}
}

func TestPickServerRoundRobinFailover(t *testing.T) {
	errBoom := errors.New("boom")
	// s0 always fails to connect; a pick must move on to the next server.
	connectFn := func(_ context.Context, sc *serverConn) error {
		if sc.address == "s0" {
			return errBoom
		}
		return nil
	}
	w := newTestWorker([]string{"s0", "s1", "s2"}, connectFn)

	sc, err := w.pickServer(context.Background())
	if err != nil {
		t.Fatalf("pickServer() error: %v", err)
	}
	if sc.address != "s1" {
		t.Errorf("expected failover to s1, got %s", sc.address)
	}
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

func TestPickServerRoundRobinAllFail(t *testing.T) {
	errBoom := errors.New("boom")
	// Every server refuses to connect: a round-robin pick must exhaust all of
	// them and return the last error instead of a server.
	connectFn := func(context.Context, *serverConn) error { return errBoom }
	w := newTestWorker([]string{"s0", "s1", "s2"}, connectFn)

	sc, err := w.pickServer(context.Background())
	if sc != nil || err == nil {
		t.Errorf("pickServer() = (%v, %v), want (nil, error)", sc, err)
	}
}

func TestPickServerStickyAllFail(t *testing.T) {
	errBoom := errors.New("boom")
	// Every server refuses to connect: the sticky pick must exhaust the shuffle
	// order and return the last error, leaving no server pinned.
	connectFn := func(context.Context, *serverConn) error { return errBoom }
	w := newTestWorker([]string{"s0", "s1", "s2"}, connectFn)
	w.shuffleFn = fixedShuffle(0, 1, 2)

	sc, err := w.pickServerSticky(context.Background())
	if sc != nil || err == nil {
		t.Errorf("pickServerSticky() = (%v, %v), want (nil, error)", sc, err)
	}
	if w.current != nil {
		t.Errorf("pickServerSticky() pinned %v despite all servers failing", w.current)
	}
}

func TestFlushSelectServerError(t *testing.T) {
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	bf := sch.NewFlowMessage()
	bf.AppendUint(schema.ColumnDstAS, 65000)
	bf.AppendUint(schema.ColumnBytes, 200)
	bf.Finalize()

	// A worker whose server selection always fails: Flush must keep retrying and
	// logging until the context expires, exercising the "cannot connect to any
	// ClickHouse server" path without ever reaching ClickHouse.
	w := &realWorker{
		c:      &realComponent{r: r, config: Configuration{}},
		bf:     bf,
		logger: r.With().Logger(),
		selectServer: func(context.Context) (*serverConn, error) {
			return nil, errors.New("no server available")
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	w.Flush(ctx)
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
