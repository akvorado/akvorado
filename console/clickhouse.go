// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"akvorado/common/httpserver"
	sb "akvorado/common/sqlbuilder"
	"akvorado/console/query"
)

// dictionary names a ClickHouse dictionary, qualified with the database it
// lives in, as the remote nodes of a cluster do not resolve a bare name.
func (c *Component) dictionary(name string) sb.TableName {
	return sb.Table(name).In(c.d.ClickHouseDB.DatabaseName())
}

// flowsTable describe a consolidated or unconsolidated flows table.
type flowsTable struct {
	Name       string
	Resolution time.Duration
	Oldest     time.Time
}

// refreshFlowsTables refreshes the information we have about flows
// tables (live one and consolidated ones). This information includes
// the consolidation interval and the oldest available data.
func (c *Component) refreshFlowsTables() error {
	ctx := c.t.Context(nil)
	var tables []struct {
		Name string `ch:"name"`
	}
	err := c.d.ClickHouseDB.Select(ctx, &tables, `
SELECT name
FROM system.tables
WHERE database=currentDatabase()
AND (table = 'flows' OR match(table, '^flows_[0-9]'))
AND table NOT LIKE '%_local'
AND (engine LIKE '%MergeTree' OR engine = 'Distributed')
`)
	if err != nil {
		return fmt.Errorf("cannot query flows table metadata: %w", err)
	}

	newFlowsTables := []flowsTable{}
	for _, table := range tables {
		// Parse resolution
		resolution := time.Duration(0)
		if strings.HasPrefix(table.Name, "flows_") {
			var err error
			resolution, err = time.ParseDuration(strings.TrimPrefix(table.Name, "flows_"))
			if err != nil {
				c.r.Err(err).Msgf("cannot parse duration for table %s", table.Name)
				continue
			}
		}
		// Get oldest timestamp
		var oldest []struct {
			T time.Time `ch:"t"`
		}
		err := c.d.ClickHouseDB.Conn.Select(ctx, &oldest,
			fmt.Sprintf(`SELECT MIN(TimeReceived) AS t FROM %s`, table.Name))
		if err != nil {
			return fmt.Errorf("cannot query table %s for oldest timestamp: %w", table.Name, err)
		}

		newFlowsTables = append(newFlowsTables, flowsTable{
			Name:       table.Name,
			Resolution: resolution,
			Oldest:     oldest[0].T,
		})
	}
	if len(newFlowsTables) == 0 {
		return errors.New("no flows table present (yet?)")
	}

	c.flowsTablesLock.Lock()
	c.flowsTables = newFlowsTables
	c.flowsTablesLock.Unlock()
	return nil
}

// flowsTablesContextKey is the key under which the flows tables known when a
// request started are stored in its context.
type flowsTablesContextKey struct{}

// contextWithFlowsTables returns a context carrying the provided flows tables.
func contextWithFlowsTables(ctx context.Context, tables []flowsTable) context.Context {
	return context.WithValue(ctx, flowsTablesContextKey{}, tables)
}

// withFlowsTables is a middleware storing the flows tables known when the
// request starts in its context. Everything serving the request then works on
// the same set of tables, even if the list is refreshed in the meantime.
func (c *Component) withFlowsTables() httpserver.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			c.flowsTablesLock.RLock()
			tables := slices.Clone(c.flowsTables)
			c.flowsTablesLock.RUnlock()
			next.ServeHTTP(w, req.WithContext(contextWithFlowsTables(req.Context(), tables)))
		})
	}
}

// flowsTablesFromContext returns the flows tables stored in the context by
// withFlowsTables(). It panics when there is none: a handler reached without
// the middleware could otherwise pick a table the authorization filter does
// not cover.
func flowsTablesFromContext(ctx context.Context) []flowsTable {
	tables, ok := ctx.Value(flowsTablesContextKey{}).([]flowsTable)
	if !ok {
		panic("flows tables missing from context: withFlowsTables() middleware not applied")
	}
	return tables
}

// inputContext describes a time range, as requested by an input handler. It is
// what the resolver needs to pick a table.
type inputContext struct {
	Start             time.Time
	End               time.Time
	MainTableRequired bool
	Points            uint
}

// resolution is the table and the interval to use for a query. It depends on
// the live table inventory. All axes of a query share the same resolution.
type resolution struct {
	// Table is the table the query should happen on.
	Table string
	// Interval is the number of seconds between two points.
	Interval uint64
	// TableInterval is the resolution of the table.
	TableInterval time.Duration
}

// resolved is a resolution applied to one time range. Axes get one each: the
// previous period covers an earlier range than the main one.
type resolved struct {
	Table string
	// Interval is the number of seconds between two points.
	Interval uint64
	// Start and End are the time range, aligned on the interval.
	Start time.Time
	End   time.Time
	// Offset shifts the buckets of toStartOfInterval, which would otherwise not
	// start on Start.
	Offset uint64
}

// unionAll combines the per-axis queries into the single statement sent to
// ClickHouse. Only the first axis carries the WITH clause, the others reference
// its CTEs.
func unionAll(queries []*sb.Query) string {
	first := queries[0]
	for _, other := range queries[1:] {
		first.UnionAll(other)
	}
	return first.String()
}

// seconds returns an interval of the provided number of seconds.
func seconds(count uint64) sb.Expr {
	return sb.Interval(sb.Uint(count), "second")
}

// dateTime turns a timestamp into a UTC date-time.
func dateTime(when time.Time) sb.Expr {
	return sb.Function("toDateTime",
		sb.String(when.UTC().Format("2006-01-02 15:04:05")),
		sb.String("UTC"))
}

// timefilterStart is the beginning of the resolved time range.
func (r resolved) timefilterStart() sb.Expr {
	return dateTime(r.Start)
}

// timefilterEnd is the end of the resolved time range.
func (r resolved) timefilterEnd() sb.Expr {
	return dateTime(r.End)
}

// timefilter restricts a query to the resolved time range.
func (r resolved) timefilter() sb.Expr {
	return sb.Between(sb.Column("TimeReceived"),
		r.timefilterStart(), r.timefilterEnd())
}

// toStartOfInterval puts TimeReceived into a bucket. The buckets are shifted so
// that the first one starts at the beginning of the time range.
func (r resolved) toStartOfInterval() sb.Expr {
	return sb.Op(
		sb.Function("toStartOfInterval",
			sb.Op(sb.Column("TimeReceived"), "+", seconds(r.Offset)),
			seconds(r.Interval)),
		"-", seconds(r.Offset))
}

// where turns a filter into a WHERE clause, restricted to the resolved time
// range.
func (r resolved) where(qf query.Filter) sb.Expr {
	return sb.And(r.timefilter(), qf.Direct())
}

// unitsExpr returns the aggregate expression matching the requested units.
// These are fixed formulas, easier to read and to check as SQL than as a tree
// of calls. They are parsed once, at startup.
func unitsExpr(units string) sb.Expr {
	return unitsExprs[units]
}

var unitsExprs = map[string]sb.Expr{
	"fps":   sb.MustParseExpr(`COUNT(*)`),
	"pps":   sb.MustParseExpr(`SUM(Packets*SamplingRate)`),
	"l3bps": sb.MustParseExpr(`SUM(Bytes*SamplingRate*8)`),
	// For each packet, we add the Ethernet header (14 bytes), the FCS (4
	// bytes), the preamble and start frame delimiter (8 bytes) and the IPG
	// (~ 12 bytes). We don't include the VLAN header (4 bytes) as it is
	// often not used with external entities. Both sFlow and IPFIX may have
	// a better view of that, but we don't collect it yet.
	"l2bps": sb.MustParseExpr(`SUM((Bytes+38*Packets)*SamplingRate*8)`),
	// That's like l2bps, but this time we use the interface speed to get a
	// percent value
	"inl2%": sb.MustParseExpr(`ifNotFinite(SUM((Bytes+38*Packets)*SamplingRate*8*100/(InIfSpeed*1000000))/COUNT(DISTINCT ExporterAddress, InIfName),0)`),
	// Same but using output interface as reference
	"outl2%": sb.MustParseExpr(`ifNotFinite(SUM((Bytes+38*Packets)*SamplingRate*8*100/(OutIfSpeed*1000000))/COUNT(DISTINCT ExporterAddress, OutIfName),0)`),
}

// unitsWeightExpr returns what one row is worth for the requested units. It is
// the value the matching aggregate in unitsExprs sums up. There is nothing for
// the percentage units, as they divide by a number of interfaces, which is not
// known from a single row.
func unitsWeightExpr(units string) sb.Expr {
	return map[string]sb.Expr{
		"fps":   sb.MustParseExpr(`1`),
		"pps":   sb.MustParseExpr(`Packets*SamplingRate`),
		"l3bps": sb.MustParseExpr(`Bytes*SamplingRate*8`),
		"l2bps": sb.MustParseExpr(`(Bytes+38*Packets)*SamplingRate*8`),
	}[units]
}

// resolve picks the best table for the requested time range and the interval
// between two points.
func (c *Component) resolve(ctx context.Context, input inputContext) resolution {
	table, tableInterval, targetInterval := c.computeTableAndInterval(ctx, input)

	// Adapt the table interval to match the target one more closely
	interval := tableInterval
	if targetInterval > tableInterval {
		interval = targetInterval.Truncate(tableInterval)
	}

	return resolution{
		Table:         table,
		Interval:      uint64(interval.Seconds()),
		TableInterval: tableInterval,
	}
}

// forRange applies a resolution to a time range, aligning it on the interval.
func (r resolution) forRange(start, end time.Time) resolved {
	interval := time.Duration(r.Interval) * time.Second
	// Make start/end match the table interval
	start = start.Truncate(r.TableInterval)
	end = end.Truncate(r.TableInterval)
	// Adapt end to ensure we get a full interval
	end = start.Add(end.Sub(start).Truncate(interval))

	return resolved{
		Table:    r.Table,
		Interval: r.Interval,
		Start:    start,
		End:      end,
		Offset:   intervalOffset(start, r.Interval),
	}
}

// shiftBack moves a resolved range back in time. Both ends move by the same
// amount, so the range keeps its length and therefore its number of points.
// This is how the previous period is drawn on the time axis of the main one.
func (r resolved) shiftBack(offset time.Duration) resolved {
	r.Start = r.Start.Add(-offset)
	r.End = r.End.Add(-offset)
	r.Offset = intervalOffset(r.Start, r.Interval)
	return r
}

// intervalOffset returns by how much the buckets of toStartOfInterval have to
// be shifted, as they would otherwise not start on the beginning of the range.
// Not using Go's truncate, but I don't remember why...
func intervalOffset(start time.Time, interval uint64) uint64 {
	seconds := int64(interval)
	aligned := time.Unix(start.UTC().Unix()/seconds*seconds, 0)
	return interval - uint64(start.UTC().Sub(aligned).Seconds())
}

func (c *Component) computeTableAndInterval(ctx context.Context, input inputContext) (string, time.Duration, time.Duration) {
	targetInterval := time.Duration(uint64(input.End.Sub(input.Start)) / uint64(input.Points))
	targetInterval = max(targetInterval, time.Second)

	// Select table
	if input.MainTableRequired {
		return "flows", time.Second, targetInterval
	}
	table, computedInterval := c.getBestTable(ctx, input.Start, targetInterval)
	return table, computedInterval, targetInterval
}

// Get the best table starting at the specified time.
func (c *Component) getBestTable(ctx context.Context, start time.Time, targetInterval time.Duration) (string, time.Duration) {
	flowsTables := flowsTablesFromContext(ctx)

	table := "flows"
	computedInterval := time.Second
	if len(flowsTables) > 0 {
		// We can use the consolidated data. The first criteria is to find the
		// tables matching the time criteria.
		candidates := []int{}
		for idx, table := range flowsTables {
			if start.After(table.Oldest.Add(table.Resolution)) {
				candidates = append(candidates, idx)
			}
		}
		if len(candidates) == 0 {
			// No candidate, fallback to the one with oldest data
			best := 0
			for idx, table := range flowsTables {
				if flowsTables[best].Oldest.After(table.Oldest.Add(table.Resolution)) {
					best = idx
				}
			}
			candidates = []int{best}
			// Add other candidates that are not far off in term of oldest data
			for idx, table := range flowsTables {
				if idx == best {
					continue
				}
				if flowsTables[best].Oldest.After(table.Oldest) {
					candidates = append(candidates, idx)
				}
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			return flowsTables[candidates[i]].Resolution < flowsTables[candidates[j]].Resolution
		})
		// If possible, use the first resolution before the target interval
		for len(candidates) > 1 {
			if flowsTables[candidates[1]].Resolution <= targetInterval {
				candidates = candidates[1:]
			} else {
				break
			}
		}
		table = flowsTables[candidates[0]].Name
		computedInterval = flowsTables[candidates[0]].Resolution
	}
	if computedInterval < time.Second {
		computedInterval = time.Second
	}
	return table, computedInterval
}
