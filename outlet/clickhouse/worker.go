// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/cenkalti/backoff/v7"

	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// Worker represents a worker sending to ClickHouse. It is synchronous (no
// goroutines) and most functions are bound to a context.
type Worker interface {
	FinalizeAndSend(context.Context) WorkerStatus
	Flush(context.Context)
	// Close releases the connections held by the worker.
	Close()
}

// WorkerStatus tells if a worker is overloaded or not.
type WorkerStatus int

const (
	// WorkerStatusIdle tells the worker is currently idle.
	WorkerStatusIdle WorkerStatus = iota
	// WorkerStatusOverloaded tells the worker has too much work and more worker would help.
	WorkerStatusOverloaded
	// WorkerStatusUnderloaded tells the worker do not have enough work.
	WorkerStatusUnderloaded
	// WorkerStatusSteady tells the worker had the right amount of work.
	WorkerStatusSteady
)

// serverConn holds, for one worker, the state of a single ClickHouse server: its
// address and a lazily-established connection.
type serverConn struct {
	address string
	conn    *ch.Client
}

// selectServerFunc picks and connects the ClickHouse server for the next batch;
// each strategy provides its own.
type selectServerFunc func(context.Context) (*serverConn, error)

// commonWorker holds everything shared by the server-selection strategies. It is
// embedded by roundRobinWorker and stickyRandomWorker, which add only the state
// their own selection needs.
type commonWorker struct {
	c      *realComponent
	bf     *schema.FlowMessage
	last   time.Time
	logger reporter.Logger

	// servers holds one entry per configured ClickHouse server.
	servers []*serverConn
	// connectFn establishes/validates the connection for a server. It defaults to
	// ensureConnected and is a field so tests can inject a fake dialer.
	connectFn     func(context.Context, *serverConn) error
	options       ch.Options
	asyncSettings []ch.Setting
}

// NewWorker creates a new worker to push data to ClickHouse.
func (c *realComponent) NewWorker(i int, bf *schema.FlowMessage) Worker {
	opts, servers := c.d.ClickHouse.ChGoOptions()
	conns := make([]*serverConn, len(servers))
	for j, s := range servers {
		conns[j] = &serverConn{address: s}
	}
	common := commonWorker{
		c:      c,
		bf:     bf,
		logger: c.r.With().Int("worker", i).Logger(),

		servers: conns,
		options: opts,
		asyncSettings: []ch.Setting{
			{
				Key:       "async_insert",
				Value:     "1",
				Important: true,
			},
			{
				Key:       "wait_for_async_insert",
				Value:     "1",
				Important: true,
			},
			{
				Key:   "async_insert_busy_timeout_max_ms",
				Value: strconv.FormatUint(uint64(c.config.MaximumWaitTime.Milliseconds()), 10),
			},
		},
	}
	if c.config.ServerSelection == ServerSelectionRoundRobin {
		// Stagger the starting server per worker so batches spread across all
		// servers from the first flush instead of all piling onto servers[0].
		w := &roundRobinWorker{commonWorker: common, next: i}
		w.connectFn = w.ensureConnected
		return w
	}
	w := &stickyRandomWorker{commonWorker: common, shuffleFn: rand.Perm}
	w.connectFn = w.ensureConnected
	return w
}

// finalizeAndSend sends data to ClickHouse after finalizing if we have a full
// batch or exceeded the maximum wait time. See
// https://clickhouse.com/docs/best-practices/selecting-an-insert-strategy for
// tips on the insert strategy. Notably, we switch to async insert when the
// batch size is too small. selectServer is the strategy's server picker.
func (w *commonWorker) finalizeAndSend(ctx context.Context, selectServer selectServerFunc) WorkerStatus {
	w.bf.Finalize()
	now := time.Now()
	batchSize := w.bf.FlowCount()
	waitTime := now.Sub(w.last)
	if batchSize >= int(w.c.config.MaximumBatchSize) || waitTime >= w.c.config.MaximumWaitTime {
		// Record wait time since last send
		if !w.last.IsZero() {
			waitTime := now.Sub(w.last)
			w.c.metrics.waitTime.Observe(waitTime.Seconds())
		}
		w.flush(ctx, selectServer)
		w.last = time.Now()
		if uint(batchSize) >= w.c.config.MaximumBatchSize {
			w.c.metrics.overloaded.Inc()
			return WorkerStatusOverloaded
		} else if uint(batchSize) <= w.c.config.minimumBatchSize {
			w.c.metrics.underloaded.Inc()
			return WorkerStatusUnderloaded
		}
		w.c.metrics.steady.Inc()
		return WorkerStatusSteady
	}
	return WorkerStatusIdle
}

// flush sends remaining data to ClickHouse without an additional condition. It
// should be called before shutting down to flush remaining data. Otherwise,
// finalizeAndSend() should be used instead.
func (w *commonWorker) flush(ctx context.Context, selectServer selectServerFunc) {
	var useAsync bool
	if w.bf.FlowCount() == 0 {
		return
	}
	// Async mode if have not a big batch size
	var settings []ch.Setting
	if uint(w.bf.FlowCount()) <= w.c.config.minimumBatchSize {
		useAsync = true
		settings = w.asyncSettings
	}

	// We try to send as long as possible. The only exit condition is an
	// expiration of the context.
	b := backoff.NewExponentialBackOff()
	b.MaxInterval = 30 * time.Second
	b.InitialInterval = 20 * time.Millisecond
	backoff.Retry(ctx, func() (any, error) {
		// Pick a server (per the configured strategy) and (re)connect if needed.
		// Depending on the strategy each batch may land on a different server
		// (round-robin) or stay on the pinned one (sticky-random).
		sc, err := selectServer(ctx)
		if err != nil {
			w.logger.Err(err).Msg("cannot connect to any ClickHouse server")
			return nil, err
		}

		// Ensure the context lives for at least GracePeriod.
		chCtx, cancel := context.WithCancel(context.Background())
		defer cancel() // needed in case the operation completes before grace period and parent context
		go func() {
			gracePeriodTimer := time.NewTimer(w.c.config.GracePeriod)
			defer gracePeriodTimer.Stop()

			select {
			case <-gracePeriodTimer.C:
				// Grace period elapsed, now wait for parent or end of operation.
				select {
				case <-ctx.Done():
				case <-chCtx.Done():
				}
			case <-ctx.Done():
				// Parent done before grace period, wait for grace period or end of operation.
				select {
				case <-gracePeriodTimer.C:
					w.logger.Info().Msg("grace period to flush batch expired")
				case <-chCtx.Done():
				}
			case <-chCtx.Done():
				// Operation done!
			}
			cancel()
		}()

		// Send to ClickHouse in flows_XXXXX_raw.
		start := time.Now()
		if err := sc.conn.Do(chCtx, ch.Query{
			Body:     w.bf.ClickHouseProtoInput().Into(fmt.Sprintf("flows_%s_raw", w.c.d.Schema.ClickHouseHash())),
			Input:    w.bf.ClickHouseProtoInput(),
			Settings: settings,
		}); err != nil {
			w.logger.Err(err).Str("server", sc.address).Int("flows", w.bf.FlowCount()).Bool("async", useAsync).Msg("cannot send batch to ClickHouse")
			w.c.metrics.errors.WithLabelValues("send").Inc()
			return nil, err
		}
		pushDuration := time.Since(start)
		w.c.metrics.insertTime.Observe(pushDuration.Seconds())
		w.c.metrics.flows.Observe(float64(w.bf.FlowCount()))
		w.c.metrics.batchesSent.WithLabelValues(sc.address).Inc()

		// Clear batch
		w.bf.Clear()
		return nil, nil
	}, backoff.WithBackOff(b), backoff.WithMaxElapsedTime(0))
}

// roundRobinWorker spreads a worker's batches across all servers in round-robin
// order.
type roundRobinWorker struct {
	commonWorker
	next int
}

// FinalizeAndSend implements Worker.
func (w *roundRobinWorker) FinalizeAndSend(ctx context.Context) WorkerStatus {
	return w.finalizeAndSend(ctx, w.pickServer)
}

// Flush implements Worker.
func (w *roundRobinWorker) Flush(ctx context.Context) { w.flush(ctx, w.pickServer) }

// pickServer selects the next ClickHouse server in round-robin order and makes
// sure it is connected, so a worker's batches are spread across all servers
// instead of pinning to one. If a server cannot be reached it moves on to the
// next one within the same call.
func (w *roundRobinWorker) pickServer(ctx context.Context) (*serverConn, error) {
	n := len(w.servers)
	var lastErr error
	for tried := 0; tried < n; tried++ {
		sc := w.servers[w.next%n]
		w.next = (w.next + 1) % n
		if err := w.connectFn(ctx, sc); err != nil {
			lastErr = err
			continue
		}
		return sc, nil
	}
	return nil, lastErr
}

// stickyRandomWorker pins the worker to a single, randomly-chosen ClickHouse
// server for the lifetime of its connection.
type stickyRandomWorker struct {
	commonWorker
	// current is the server the worker is pinned to.
	current *serverConn
	// shuffleFn returns a random permutation of [0,n) and is used to pick a
	// server. It defaults to rand.Perm and is a field so tests can make the
	// choice deterministic.
	shuffleFn func(int) []int
}

// FinalizeAndSend implements Worker.
func (w *stickyRandomWorker) FinalizeAndSend(ctx context.Context) WorkerStatus {
	return w.finalizeAndSend(ctx, w.pickServer)
}

// Flush implements Worker.
func (w *stickyRandomWorker) Flush(ctx context.Context) { w.flush(ctx, w.pickServer) }

// pickServer keeps using the pinned server as long as its connection stays
// healthy. A new server is picked (at random, via shuffleFn) only when there is
// no pinned server or the pinned connection broke, reproducing the historical
// rand.Perm selection.
func (w *stickyRandomWorker) pickServer(ctx context.Context) (*serverConn, error) {
	// Reuse the pinned server while its connection is (or can be made) healthy.
	if w.current != nil {
		if err := w.connectFn(ctx, w.current); err == nil {
			return w.current, nil
		}
		// The connection broke: drop the pin and re-pick a different one below.
		w.current = nil
	}

	// Pick a new server at random.
	var lastErr error
	for _, idx := range w.shuffleFn(len(w.servers)) {
		sc := w.servers[idx]
		if err := w.connectFn(ctx, sc); err != nil {
			lastErr = err
			continue
		}
		w.current = sc
		return sc, nil
	}
	return nil, lastErr
}

// ensureConnected makes sure sc has a healthy connection, dialing (or redialing)
// as needed.
func (w *commonWorker) ensureConnected(ctx context.Context, sc *serverConn) error {
	// If connection exists and is healthy, reuse it.
	if sc.conn != nil {
		if err := sc.conn.Ping(ctx); err == nil {
			return nil
		}
		sc.conn.Close()
		sc.conn = nil
	}

	// Dial this specific server. Copy options so the per-server address does not
	// clobber the shared template.
	opts := w.options
	opts.Address = sc.address
	conn, err := ch.Dial(ctx, opts)
	if err != nil {
		w.logger.Err(err).Str("server", sc.address).Msg("failed to connect to ClickHouse server")
		w.c.metrics.errors.WithLabelValues("connect").Inc()
		return err
	}
	if err := conn.Ping(ctx); err != nil {
		w.logger.Err(err).Str("server", sc.address).Msg("ClickHouse server ping failed")
		w.c.metrics.errors.WithLabelValues("ping").Inc()
		conn.Close()
		return err
	}
	sc.conn = conn
	w.logger.Info().Str("server", sc.address).Msg("connected to ClickHouse server")
	return nil
}

// Close releases every connection this worker opened.
func (w *commonWorker) Close() {
	for _, sc := range w.servers {
		if sc.conn != nil {
			sc.conn.Close()
			sc.conn = nil
		}
	}
}
