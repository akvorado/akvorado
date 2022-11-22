// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"
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

// finalizeQuery builds the finalized query. A single "context"
// function is provided to return a `Context` struct with all the
// information needed.
func (c *Component) finalizeQuery(query string) string {
	t := template.Must(template.New("query").
		Funcs(template.FuncMap{
			"context": c.contextFunc,
		}).
		Option("missingkey=error").
		Parse(strings.TrimSpace(query)))
	buf := bytes.NewBufferString("")
	if err := t.Execute(buf, nil); err != nil {
		c.r.Err(err).Str("query", query).Msg("invalid query")
		panic(err)
	}
	return buf.String()
}

type inputContext struct {
	Start             time.Time  `json:"start"`
	End               time.Time  `json:"end"`
	StartForInterval  *time.Time `json:"start-for-interval,omitempty"`
	MainTableRequired bool       `json:"main-table-required,omitempty"`
	Points            uint       `json:"points"`
	Units             string     `json:"units,omitempty"`
}

type context struct {
	Table             string
	Timefilter        string
	TimefilterStart   string
	TimefilterEnd     string
	Units             string
	Interval          uint64
	ToStartOfInterval func(string) string
}

// templateEscape escapes `{{` and `}}` from a string. In fact, only
// the opening tag needs to be escaped.
func templateEscape(input string) string {
	return strings.ReplaceAll(input, `{{`, `{{"{{"}}`)
}

// templateWhere transforms a filter to a WHERE clause
func templateWhere(qf queryFilter) string {
	if qf.Filter == "" {
		return `{{ .Timefilter }}`
	}
	return fmt.Sprintf(`{{ .Timefilter }} AND (%s)`, templateEscape(qf.Filter))
}

// templateTable builds a template directive to select the right table
func templateContext(context inputContext) string {
	encoded, err := json.Marshal(context)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("context `%s`", string(encoded))
}

func (c *Component) contextFunc(inputStr string) context {
	var input inputContext
	if err := json.Unmarshal([]byte(inputStr), &input); err != nil {
		panic(err)
	}

	targetInterval := time.Duration(uint64(input.End.Sub(input.Start)) / uint64(input.Points))
	if targetInterval < time.Second {
		targetInterval = time.Second
	}

	// Select table
	targetIntervalForTableSelection := targetInterval
	if input.MainTableRequired {
		targetIntervalForTableSelection = time.Second
	}
	table, computedInterval := c.getBestTable(input.Start, targetIntervalForTableSelection)
	if input.StartForInterval != nil {
		_, computedInterval = c.getBestTable(*input.StartForInterval, targetIntervalForTableSelection)
	}

	// Make start/end match the computed interval (currently equal to the table resolution)
	start := input.Start.Truncate(computedInterval)
	end := input.End.Truncate(computedInterval)
	// Adapt the computed interval to match the target one more closely
	if targetInterval > computedInterval {
		computedInterval = targetInterval.Truncate(computedInterval)
	}
	// Adapt end to ensure we get a full interval
	end = start.Add(end.Sub(start).Truncate(computedInterval))
	// Now, toStartOfInterval will provide an incorrect value. We
	// compute a correction offset. Go's truncate seems to
	// be different from what we expect.
	computedIntervalOffset := start.UTC().Sub(
		time.Unix(start.UTC().Unix()/
			int64(computedInterval.Seconds())*
			int64(computedInterval.Seconds()), 0))
	diffOffset := uint64(computedInterval.Seconds()) - uint64(computedIntervalOffset.Seconds())

	// Compute all strings
	timefilterStart := fmt.Sprintf(`toDateTime('%s', 'UTC')`, start.UTC().Format("2006-01-02 15:04:05"))
	timefilterEnd := fmt.Sprintf(`toDateTime('%s', 'UTC')`, end.UTC().Format("2006-01-02 15:04:05"))
	timefilter := fmt.Sprintf(`TimeReceived BETWEEN %s AND %s`, timefilterStart, timefilterEnd)
	var units string
	switch input.Units {
	case "pps":
		units = `SUM(Packets*SamplingRate)`
	case "l3bps":
		units = `SUM(Bytes*SamplingRate*8)`
	case "l2bps":
		units = `SUM((Bytes+18*Packets)*SamplingRate*8)`
	}

	c.metrics.clickhouseQueries.WithLabelValues(table).Inc()
	return context{
		Table:           table,
		Timefilter:      timefilter,
		TimefilterStart: timefilterStart,
		TimefilterEnd:   timefilterEnd,
		Units:           units,
		Interval:        uint64(computedInterval.Seconds()),
		ToStartOfInterval: func(field string) string {
			return fmt.Sprintf(
				`toStartOfInterval(%s + INTERVAL %d second, INTERVAL %d second) - INTERVAL %d second`,
				field,
				diffOffset,
				uint64(computedInterval.Seconds()),
				diffOffset)
		},
	}
}

// Get the best table starting at the specified time.
func (c *Component) getBestTable(start time.Time, targetInterval time.Duration) (string, time.Duration) {
	c.flowsTablesLock.RLock()
	defer c.flowsTablesLock.RUnlock()

	table := "flows"
	computedInterval := time.Second
	if len(c.flowsTables) > 0 {
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
		sort.Slice(candidates, func(i, j int) bool {
			return c.flowsTables[candidates[i]].Resolution < c.flowsTables[candidates[j]].Resolution
		})
		// If possible, use the first resolution before the target interval
		for len(candidates) > 1 {
			if c.flowsTables[candidates[1]].Resolution < targetInterval {
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
