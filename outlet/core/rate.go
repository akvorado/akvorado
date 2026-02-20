// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"net/netip"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"golang.org/x/time/rate"
)

// rateLimiter tracks per-exporter rate limiting state.
type rateLimiter struct {
	*xsync.Map[netip.Addr, perExporterRateLimiter]
}

type perExporterRateLimiter struct {
	l           *rate.Limiter
	dropped     uint64  // dropped during the current second
	total       uint64  // total during the current second
	dropRate    float64 // drop rate during the last second
	currentTick time.Time
}

// newRateLimiter returns a new per-exporter rate limiter.
func newRateLimiter() rateLimiter {
	return rateLimiter{
		Map: xsync.NewMap[netip.Addr, perExporterRateLimiter](),
	}
}

// allowOneMessage checks if a flow from the given exporter should be allowed,
// given the configured rateLimit (flows/s). It returns the drop rate which can
// be used to adjust the sampling rate. rateLimit is assumed to be > 0.
func (rl rateLimiter) allowOneMessage(exporter netip.Addr, rateLimit uint64) (bool, float64) {
	now := time.Now()
	tick := now.Truncate(200 * time.Millisecond) // we use a 200-millisecond resolution
	verdict := true
	update := func(value perExporterRateLimiter, loaded bool) (perExporterRateLimiter, xsync.ComputeOp) {
		if !loaded {
			value = perExporterRateLimiter{
				l:           rate.NewLimiter(rate.Limit(rateLimit), int(rateLimit/10)),
				currentTick: now,
			}
		}
		if value.currentTick.UnixMilli() != tick.UnixMilli() {
			value.dropRate = float64(value.dropped) / float64(value.total)
			value.dropped = 0
			value.total = 0
			value.currentTick = tick
		}
		value.total++
		if !value.l.AllowN(now, 1) {
			value.dropped++
			verdict = false
		}
		return value, xsync.UpdateOp
	}
	value, _ := rl.Compute(exporter, update)
	return verdict, value.dropRate
}
