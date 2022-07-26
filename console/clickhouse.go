// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	addressOrPortRegexp = regexp.MustCompile(`\b(?:Src|Dst)(?:Port|Addr)\b`)
	resolutionRegexp    = regexp.MustCompile(`{resolution->(\d+)}`)
)

// flowsTable describe a consolidated or unconsolidated flows table.
type flowsTable struct {
	Name       string
	Resolution time.Duration
	Oldest     time.Time
}

// Build a query against the flows table or one of the consolidated
// version depending on the information needed. The provided query
// should contain `{table}` which will be replaced by the appropriate
// flows table and {timefilter} which will be replaced by the
// appropriate time filter.
func (c *Component) queryFlowsTable(query string, start, end time.Time, targetResolution time.Duration) string {
	c.flowsTablesLock.RLock()
	defer c.flowsTablesLock.RUnlock()

	// Select table
	table := "flows"
	resolution := time.Second
	if !addressOrPortRegexp.MatchString(query) {
		// We can use the consolidated data. The first
		// criteria is to find the tables matching the time
		// criteria.
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
		if len(candidates) > 1 {
			// Use resolution to find the best one
			best := 0
			for _, idx := range candidates {
				if c.flowsTables[idx].Resolution > targetResolution {
					continue
				}
				if c.flowsTables[idx].Resolution > c.flowsTables[best].Resolution {
					best = idx
				}
			}
			candidates = []int{best}
		}
		table = c.flowsTables[candidates[0]].Name
		resolution = c.flowsTables[candidates[0]].Resolution
	}
	if resolution == 0 {
		resolution = time.Second
	}

	// Build timefilter to match the resolution
	start = start.Truncate(resolution)
	end = end.Truncate(resolution)
	timeFilterStart := fmt.Sprintf(`toDateTime('%s', 'UTC')`, start.UTC().Format("2006-01-02 15:04:05"))
	timeFilterStop := fmt.Sprintf(`toDateTime('%s', 'UTC')`, end.UTC().Format("2006-01-02 15:04:05"))
	timeFilter := fmt.Sprintf(`TimeReceived BETWEEN %s AND %s`, timeFilterStart, timeFilterStop)

	c.metrics.clickhouseQueries.WithLabelValues(table).Inc()
	query = strings.ReplaceAll(query, "{timefilter}", timeFilter)
	query = strings.ReplaceAll(query, "{timefilter.Start}", timeFilterStart)
	query = strings.ReplaceAll(query, "{timefilter.Stop}", timeFilterStop)
	query = strings.ReplaceAll(query, "{table}", table)
	query = strings.ReplaceAll(query, "{resolution}", strconv.Itoa(int(resolution.Seconds())))
	query = resolutionRegexp.ReplaceAllStringFunc(query, func(in string) string {
		matches := resolutionRegexp.FindStringSubmatch(in)
		target, err := strconv.Atoi(matches[1])
		if err != nil {
			panic(err)
		}
		target = target / int(resolution.Seconds()) * int(resolution.Seconds())
		if target < 1 {
			target = 1
		}
		return strconv.Itoa(target)
	})
	return query
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
AND engine LIKE '%MergeTree'
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
