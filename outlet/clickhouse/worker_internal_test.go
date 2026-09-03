// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"errors"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/ClickHouse/ch-go"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

func newCommon(addrs []string, connectFn func(context.Context, *serverConn) error) commonWorker {
	conns := make([]*serverConn, len(addrs))
	for i, a := range addrs {
		conns[i] = &serverConn{address: a}
	}
	return commonWorker{servers: conns, connectFn: connectFn}
}

func newRoundRobin(addrs []string, connectFn func(context.Context, *serverConn) error) *roundRobinWorker {
	w := &roundRobinWorker{commonWorker: newCommon(addrs, connectFn)}
	w.selectServer = w.pickServer
	return w
}

func newSticky(addrs []string, connectFn func(context.Context, *serverConn) error) *stickyRandomWorker {
	w := &stickyRandomWorker{commonWorker: newCommon(addrs, connectFn), shuffleFn: rand.Perm}
	w.selectServer = w.pickServer
	return w
}

// fixedShuffle returns a shuffleFn that always yields the given permutation,
// making the sticky-random pick deterministic in tests.
func fixedShuffle(order ...int) func(int) []int {
	return func(int) []int { return order }
}

// pickN calls pick n times and returns the sequence of selected addresses.
func pickN(t *testing.T, pick func(context.Context) (*serverConn, error), n int) []string {
	t.Helper()
	got := make([]string, 0, n)
	for range n {
		sc, err := pick(context.Background())
		if err != nil {
			t.Fatalf("pickServer() error:\n%v", err)
		}
		got = append(got, sc.address)
	}
	return got
}

func TestPickServerRoundRobin(t *testing.T) {
	ok := func(context.Context, *serverConn) error { return nil }
	w := newRoundRobin([]string{"s0", "s1", "s2"}, ok)

	got := pickN(t, w.pickServer, 7)
	want := []string{"s0", "s1", "s2", "s0", "s1", "s2", "s0"}
	if diff := helpers.Diff(got, want); diff != "" {
		t.Errorf("round-robin order (-got, +want):\n%s", diff)
	}
}

func TestPickServerStaggeredStart(t *testing.T) {
	ok := func(context.Context, *serverConn) error { return nil }
	// A worker whose cursor starts at index 1 (staggered by worker id) should
	// begin on s1, not s0.
	w := newRoundRobin([]string{"s0", "s1", "s2"}, ok)
	w.next = 1

	got := pickN(t, w.pickServer, 3)
	want := []string{"s1", "s2", "s0"}
	if diff := helpers.Diff(got, want); diff != "" {
		t.Errorf("staggered start (-got, +want):\n%s", diff)
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
	w := newRoundRobin([]string{"s0", "s1", "s2"}, connectFn)

	sc, err := w.pickServer(context.Background())
	if err != nil {
		t.Fatalf("pickServer() error:\n%v", err)
	}
	if diff := helpers.Diff(sc.address, "s1"); diff != "" {
		t.Errorf("pickServer() (-got, +want):\n%s", diff)
	}
}

func TestPickServerStickyStaysPinned(t *testing.T) {
	ok := func(context.Context, *serverConn) error { return nil }
	w := newSticky([]string{"s0", "s1", "s2"}, ok)
	// The random pick lands on s2 first; the worker must then stay on it.
	w.shuffleFn = fixedShuffle(2, 0, 1)

	got := pickN(t, w.pickServer, 5)
	want := []string{"s2", "s2", "s2", "s2", "s2"}
	if diff := helpers.Diff(got, want); diff != "" {
		t.Errorf("sticky pin (-got, +want):\n%s", diff)
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
	w := newSticky([]string{"s0", "s1", "s2"}, connectFn)
	w.shuffleFn = fixedShuffle(0, 1, 2)

	got := pickN(t, w.pickServer, 3)
	want := []string{"s1", "s1", "s1"}
	if diff := helpers.Diff(got, want); diff != "" {
		t.Errorf("sticky initial failover (-got, +want):\n%s", diff)
	}
}

func TestPickServerRoundRobinAllFail(t *testing.T) {
	errBoom := errors.New("boom")
	// Every server refuses to connect: a round-robin pick must exhaust all of
	// them and return the last error instead of a server.
	connectFn := func(context.Context, *serverConn) error { return errBoom }
	w := newRoundRobin([]string{"s0", "s1", "s2"}, connectFn)

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
	w := newSticky([]string{"s0", "s1", "s2"}, connectFn)
	w.shuffleFn = fixedShuffle(0, 1, 2)

	sc, err := w.pickServer(context.Background())
	if sc != nil || err == nil {
		t.Errorf("pickServer() = (%v, %v), want (nil, error)", sc, err)
	}
	if w.current != nil {
		t.Errorf("pickServer() pinned %v despite all servers failing", w.current)
	}
}

func TestFlushSelectServerError(t *testing.T) {
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	bf := sch.NewFlowMessage()
	bf.AppendUint(schema.ColumnDstAS, 65000)
	bf.AppendUint(schema.ColumnBytes, 200)
	bf.Finalize()

	// A worker whose every server refuses to connect: Flush must keep retrying
	// until the context expires and never reach ClickHouse.
	connectFn := func(context.Context, *serverConn) error { return errors.New("no server available") }
	w := newRoundRobin([]string{"s0"}, connectFn)
	w.c = &realComponent{r: r, config: Configuration{}}
	w.bf = bf
	w.logger = r.With().Logger()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	w.Flush(ctx)

	// Nothing could be sent, so the batch must be preserved (no data lost).
	if got := w.bf.FlowCount(); got == 0 {
		t.Errorf("Flush() cleared the batch despite no reachable server (FlowCount()=%d)", got)
	}
}

func TestConnectErrorMetric(t *testing.T) {
	r := reporter.NewMock(t)
	c := &realComponent{r: r, config: DefaultConfiguration()}
	c.initMetrics()
	// A single unreachable server (invalid port 0): a pick fails and must count
	// exactly one connect error.
	w := &roundRobinWorker{commonWorker: commonWorker{
		c:       c,
		logger:  r.With().Logger(),
		servers: []*serverConn{{address: "127.0.0.1:0"}},
		options: ch.Options{DialTimeout: 100 * time.Millisecond},
	}}
	w.connectFn = w.ensureConnected

	if sc, err := w.pickServer(context.Background()); sc != nil || err == nil {
		t.Fatalf("pickServer() = (%v, %v), want (nil, error)", sc, err)
	}
	gotMetrics := r.GetMetrics("akvorado_outlet_clickhouse_", "errors_total")
	expected := map[string]string{`errors_total{error="connect"}`: "1"}
	if diff := helpers.Diff(gotMetrics, expected); diff != "" {
		t.Errorf("connect error metric (-got, +want):\n%s", diff)
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
	w := newSticky([]string{"s0", "s1", "s2"}, connectFn)
	w.shuffleFn = fixedShuffle(0, 1, 2)

	// Pin to s0, then make its connection start failing.
	if got := pickN(t, w.pickServer, 1); got[0] != "s0" {
		t.Fatalf("expected initial pin to s0, got %s", got[0])
	}
	down["s0"] = true

	// The reuse attempt fails, so the worker drops the pin and re-picks the next
	// healthy server.
	sc, err := w.pickServer(context.Background())
	if err != nil {
		t.Fatalf("pickServer() error:\n%v", err)
	}
	if diff := helpers.Diff(sc.address, "s1"); diff != "" {
		t.Errorf("re-pick after broken connection (-got, +want):\n%s", diff)
	}
}
