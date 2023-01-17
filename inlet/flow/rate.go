// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"time"

	"akvorado/common/schema"

	"golang.org/x/time/rate"
)

type limiter struct {
	l           *rate.Limiter
	dropped     uint64  // dropped during the current second
	total       uint64  // total during the current second
	dropRate    float64 // drop rate during the last second
	currentTick time.Time
}

// allowMessages tell if we can transmit the provided messages,
// depending on the rate limiter configuration. If yes, their sampling
// rate may be modified to match current drop rate.
func (c *Component) allowMessages(fmsgs []*schema.FlowMessage) bool {
	count := len(fmsgs)
	if c.config.RateLimit == 0 || count == 0 {
		return true
	}
	exporter := fmsgs[0].ExporterAddress
	exporterLimiter, ok := c.limiters[exporter]
	if !ok {
		exporterLimiter = &limiter{
			l: rate.NewLimiter(rate.Limit(c.config.RateLimit), int(c.config.RateLimit/10)),
		}
		c.limiters[exporter] = exporterLimiter
	}
	now := time.Now()
	tick := now.Truncate(200 * time.Millisecond) // we use a 200-millisecond resolution
	if exporterLimiter.currentTick.UnixMilli() != tick.UnixMilli() {
		exporterLimiter.dropRate = float64(exporterLimiter.dropped) / float64(exporterLimiter.total)
		exporterLimiter.dropped = 0
		exporterLimiter.total = 0
		exporterLimiter.currentTick = tick
	}
	exporterLimiter.total += uint64(count)
	if !exporterLimiter.l.AllowN(now, count) {
		exporterLimiter.dropped += uint64(count)
		return false
	}
	if exporterLimiter.dropRate > 0 {
		for _, flow := range fmsgs {
			flow.SamplingRate *= uint32(1 / (1 - exporterLimiter.dropRate))
		}
	}
	return true
}
