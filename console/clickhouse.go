// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/template"
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

// inputContext is the intermeidate context provided by the input handler.
type inputContext struct {
	Start                  time.Time
	End                    time.Time
	StartForTableSelection *time.Time
	MainTableRequired      bool
	Points                 uint
	Units                  string
}

// context is the context to finalize the template.
type context struct {
	Table             string
	Timefilter        string
	TimefilterStart   string
	TimefilterEnd     string
	Units             string
	Interval          uint64
	ToStartOfInterval func(string) string
}

// templateQuery holds a template string and its associated input context.
type templateQuery struct {
	Template string
	Context  inputContext
}

// templateEscape escapes `{{` and `}}` from a string. In fact, only
// the opening tag needs to be escaped.
func templateEscape(input string) string {
	return strings.ReplaceAll(input, `{{`, `{{"{{"}}`)
}

// templateWhere transforms a filter to a WHERE clause
func templateWhere(qf query.Filter) string {
	if qf.Direct() == "" {
		return `{{ .Timefilter }}`
	}
	return fmt.Sprintf(`{{ .Timefilter }} AND (%s)`, templateEscape(qf.Direct()))
}

// finalizeTemplateQueries builds the finalized queries from a list of templateQuery.
// Each template is processed with its associated context and combined with UNION ALL.
func (c *Component) finalizeTemplateQueries(queries []templateQuery) string {
	parts := make([]string, len(queries))
	for i, q := range queries {
		parts[i] = c.finalizeTemplateQuery(q)
	}
	return strings.Join(parts, "\nUNION ALL\n")
}

// finalizeTemplateQuery builds the finalized query for a single templateQuery
func (c *Component) finalizeTemplateQuery(query templateQuery) string {
	input := query.Context
	table, computedInterval, targetInterval := c.computeTableAndInterval(query.Context)

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
		// For each packet, we add the Ethernet header (14 bytes), the FCS (4
		// bytes), the preamble and start frame delimiter (8 bytes) and the IPG
		// (~ 12 bytes). We don't include the VLAN header (4 bytes) as it is
		// often not used with external entities. Both sFlow and IPFIX may have
		// a better view of that, but we don't collect it yet.
		units = `SUM((Bytes+38*Packets)*SamplingRate*8)`
	case "inl2%":
		// That's like l2bps, but this time we use the interface speed to get a
		// percent value
		units = `ifNotFinite(SUM((Bytes+38*Packets)*SamplingRate*8*100/(InIfSpeed*1000000))/COUNT(DISTINCT ExporterAddress, InIfName),0)`
	case "outl2%":
		// Same but using output interface as reference
		units = `ifNotFinite(SUM((Bytes+38*Packets)*SamplingRate*8*100/(OutIfSpeed*1000000))/COUNT(DISTINCT ExporterAddress, OutIfName),0)`
	}

	c.metrics.clickhouseQueries.WithLabelValues(table).Inc()

	context := context{
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

	t := template.Must(template.New("query").
		Option("missingkey=error").
		Parse(strings.TrimSpace(query.Template)))
	buf := bytes.NewBufferString("")
	if err := t.Execute(buf, context); err != nil {
		c.r.Err(err).Str("query", query.Template).Msg("invalid query")
		panic(err)
	}
	return buf.String()
}

func (c *Component) computeTableAndInterval(input inputContext) (string, time.Duration, time.Duration) {
	targetInterval := time.Duration(uint64(input.End.Sub(input.Start)) / uint64(input.Points))
	targetInterval = max(targetInterval, time.Second)

	// Select table
	targetIntervalForTableSelection := targetInterval
	if input.MainTableRequired {
		targetIntervalForTableSelection = time.Second
	}
	startForTableSelection := input.Start
	if input.StartForTableSelection != nil {
		startForTableSelection = *input.StartForTableSelection
	}
	table, computedInterval := c.getBestTable(startForTableSelection, targetIntervalForTableSelection)
	return table, computedInterval, targetInterval
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
