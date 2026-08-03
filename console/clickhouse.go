// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

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
	Table             string
	Interval          uint64
	Timefilter        string
	TimefilterStart   string
	TimefilterEnd     string
	ToStartOfInterval string
}

// unionAll combines the per-axis queries into the single statement sent to
// ClickHouse. Only the first axis carries the WITH clause, the others reference
// its CTEs.
func unionAll(queries []string) string {
	return strings.Join(queries, "\nUNION ALL\n")
}

// where turns a filter into a WHERE clause, restricted to the resolved time
// range.
func (r resolved) where(qf query.Filter) string {
	if qf.Direct() == "" {
		return r.Timefilter
	}
	return fmt.Sprintf(`%s AND (%s)`, r.Timefilter, qf.Direct())
}

// unitsExpr returns the aggregate expression matching the requested units.
func unitsExpr(units string) string {
	return map[string]string{
		"fps":   `COUNT(*)`,
		"pps":   `SUM(Packets*SamplingRate)`,
		"l3bps": `SUM(Bytes*SamplingRate*8)`,
		// For each packet, we add the Ethernet header (14 bytes), the FCS (4
		// bytes), the preamble and start frame delimiter (8 bytes) and the IPG
		// (~ 12 bytes). We don't include the VLAN header (4 bytes) as it is
		// often not used with external entities. Both sFlow and IPFIX may have
		// a better view of that, but we don't collect it yet.
		"l2bps": `SUM((Bytes+38*Packets)*SamplingRate*8)`,
		// That's like l2bps, but this time we use the interface speed to get a
		// percent value
		"inl2%": `ifNotFinite(SUM((Bytes+38*Packets)*SamplingRate*8*100/(InIfSpeed*1000000))/COUNT(DISTINCT ExporterAddress, InIfName),0)`,
		// Same but using output interface as reference
		"outl2%": `ifNotFinite(SUM((Bytes+38*Packets)*SamplingRate*8*100/(OutIfSpeed*1000000))/COUNT(DISTINCT ExporterAddress, OutIfName),0)`,
	}[units]
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
	// Now, toStartOfInterval will provide an incorrect value. We compute a
	// correction offset. Go's truncate seems to be different from what we
	// expect.
	intervalOffset := start.UTC().Sub(
		time.Unix(start.UTC().Unix()/
			int64(interval.Seconds())*
			int64(interval.Seconds()), 0))
	diffOffset := r.Interval - uint64(intervalOffset.Seconds())

	timefilterStart := fmt.Sprintf(`toDateTime('%s', 'UTC')`, start.UTC().Format("2006-01-02 15:04:05"))
	timefilterEnd := fmt.Sprintf(`toDateTime('%s', 'UTC')`, end.UTC().Format("2006-01-02 15:04:05"))

	return resolved{
		Table:           r.Table,
		Interval:        r.Interval,
		Timefilter:      fmt.Sprintf(`TimeReceived BETWEEN %s AND %s`, timefilterStart, timefilterEnd),
		TimefilterStart: timefilterStart,
		TimefilterEnd:   timefilterEnd,
		ToStartOfInterval: fmt.Sprintf(
			`toStartOfInterval(%s + INTERVAL %d second, INTERVAL %d second) - INTERVAL %d second`,
			"TimeReceived",
			diffOffset,
			r.Interval,
			diffOffset),
	}
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
