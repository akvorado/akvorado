// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	sb "akvorado/common/sqlbuilder"
	"akvorado/console/query"
)

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
AND table LIKE 'flows%'
AND table NOT LIKE '%_local'
AND table != 'flows_raw_errors'
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

// resolve picks the best table for the requested time range and the interval
// between two points.
func (c *Component) resolve(input inputContext) resolution {
	table, tableInterval, targetInterval := c.computeTableAndInterval(input)

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

func (c *Component) computeTableAndInterval(input inputContext) (string, time.Duration, time.Duration) {
	targetInterval := time.Duration(uint64(input.End.Sub(input.Start)) / uint64(input.Points))
	targetInterval = max(targetInterval, time.Second)

	// Select table
	if input.MainTableRequired {
		return "flows", time.Second, targetInterval
	}
	table, computedInterval := c.getBestTable(input.Start, targetInterval)
	return table, computedInterval, targetInterval
}

// Get the best table starting at the specified time.
func (c *Component) getBestTable(start time.Time, targetInterval time.Duration) (string, time.Duration) {
	c.flowsTablesLock.RLock()
	defer c.flowsTablesLock.RUnlock()

	table := "flows"
	computedInterval := time.Second
	if len(c.flowsTables) > 0 {
		// We can use the consolidated data. The first criteria is to find the
		// tables matching the time criteria.
		candidates := []int{}
		for idx, table := range c.flowsTables {
			if start.After(table.Oldest.Add(table.Resolution)) {
				candidates = append(candidates, idx)
			}
		}
		if len(candidates) == 0 {
			// No candidate, fallback to the one with oldest data
			best := 0
			for idx, table := range c.flowsTables {
				if c.flowsTables[best].Oldest.After(table.Oldest.Add(table.Resolution)) {
					best = idx
				}
			}
			candidates = []int{best}
			// Add other candidates that are not far off in term of oldest data
			for idx, table := range c.flowsTables {
				if idx == best {
					continue
				}
				if c.flowsTables[best].Oldest.After(table.Oldest) {
					candidates = append(candidates, idx)
				}
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			return c.flowsTables[candidates[i]].Resolution < c.flowsTables[candidates[j]].Resolution
		})
		// If possible, use the first resolution before the target interval
		for len(candidates) > 1 {
			if c.flowsTables[candidates[1]].Resolution <= targetInterval {
				candidates = candidates[1:]
			} else {
				break
			}
		}
		table = c.flowsTables[candidates[0]].Name
		computedInterval = c.flowsTables[candidates[0]].Resolution
	}
	if computedInterval < time.Second {
		computedInterval = time.Second
	}
	return table, computedInterval
}
