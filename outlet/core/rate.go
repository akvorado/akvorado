// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"net/netip"

	"github.com/puzpuzpuz/xsync/v4"
)

// rateLimiter tracks per-exporter rate limiting state.
type rateLimiter struct {
	*xsync.Map[netip.Addr, perExporterRateLimiter]
}

type perExporterRateLimiter struct {
	dropped       uint64  // dropped during the current second
	total         uint64  // received during the current second
	factor        float64 // sampling rate correction computed from the last second
	currentSecond uint64  // second the two counters above apply to
}

// newRateLimiter returns a new per-exporter rate limiter.
func newRateLimiter() rateLimiter {
	return rateLimiter{
		Map: xsync.NewMap[netip.Addr, perExporterRateLimiter](),
	}
}

// allowOneMessage checks if a flow from the given exporter should be allowed,
// given the configured rateLimit (flows/s). timeReceived is the second the
// inlet got the flow. It returns the factor the sampling rate should be
// multiplied by to compensate for the dropped flows. rateLimit is assumed to be
// > 0.
func (rl rateLimiter) allowOneMessage(exporter netip.Addr, rateLimit, timeReceived uint64) (bool, float64) {
	verdict := true
	update := func(value perExporterRateLimiter, loaded bool) (perExporterRateLimiter, xsync.ComputeOp) {
		if !loaded {
			value = perExporterRateLimiter{factor: 1, currentSecond: timeReceived}
		}
		if timeReceived > value.currentSecond {
			// The accepted flows stand for the dropped ones, so their sampling
			// rate is multiplied by the ratio between what was received and
			// what was accepted.
			if accepted := value.total - value.dropped; accepted > 0 {
				value.factor = float64(value.total) / float64(accepted)
			}
			value.dropped = 0
			value.total = 0
			value.currentSecond = timeReceived
		}
		// A flow older than the current second is counted with it. Kafka
		// partitions are read in parallel, so flows do not always come in
		// order.
		value.total++
		if value.total > rateLimit {
			value.dropped++
			verdict = false
		}
		return value, xsync.UpdateOp
	}
	value, _ := rl.Compute(exporter, update)
	return verdict, value.factor
}
