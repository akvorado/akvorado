// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/uptrace/bun"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// queryHook is a bun.QueryHook bridging bun query events to the reporter logger.
type queryHook struct {
	r *reporter.Reporter
}

func newQueryHook(r *reporter.Reporter) *queryHook {
	return &queryHook{r: r}
}

// BeforeQuery is called before a query is executed.
func (h *queryHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

// AfterQuery is called after a query is executed.
func (h *queryHook) AfterQuery(_ context.Context, event *bun.QueryEvent) {
	fields := helpers.M{
		"sql":      event.Query,
		"duration": time.Since(event.StartTime),
	}
	if event.Err != nil && !errors.Is(event.Err, sql.ErrNoRows) {
		h.r.Err(event.Err).Fields(fields).Msg("SQL query error")
		return
	}
	h.r.Debug().Fields(fields).Msg("SQL query")
}
