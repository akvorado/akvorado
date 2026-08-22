// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	sb "akvorado/common/sqlbuilder"
	"akvorado/console/query"
)

// graphLineHandlerInput describes the input for the /graph/line endpoint.
type graphLineHandlerInput struct {
	graphCommonHandlerInput
	Points         uint `json:"points" validate:"required,min=5,max=2000"` // minimum number of points
	Bidirectional  bool `json:"bidirectional"`
	PreviousPeriod bool `json:"previous-period"`
}

// graphLineHandlerOutput describes the output for the /graph/line endpoint. A
// row is a set of values for dimensions. Currently, axis 1 is for the
// direct direction and axis 2 is for the reverse direction. Rows are
// sorted by axis, then by the sum of traffic.
type graphLineHandlerOutput struct {
	Time                 []time.Time    `json:"t"`
	Rows                 [][]string     `json:"rows"`   // List of rows
	Points               [][]int        `json:"points"` // t → row → xps
	Axis                 []int          `json:"axis"`   // row → axis
	AxisNames            map[int]string `json:"axis-names"`
	Average              []int          `json:"average"`         // row → average xps
	Total                []int          `json:"total,omitempty"` // row → total (bytes/packets/frames)
	Min                  []int          `json:"min"`             // row → min xps
	Max                  []int          `json:"max"`             // row → max xps
	Last                 []int          `json:"last"`            // row → last xps
	NinetyFivePercentile []int          `json:"95th"`            // row → 95th xps
}

// reverseDirection reverts the direction of a provided input. It does not
// modify the original.
func (input graphLineHandlerInput) reverseDirection() graphLineHandlerInput {
	input.Filter.Swap()
	input.Dimensions = slices.Clone(input.Dimensions)
	query.Columns(input.Dimensions).Reverse(input.schema)
	return input
}

// nearestPeriod returns the name and period matching the provided
// period length. The year is a special case as we don't know its
// exact length.
func nearestPeriod(period time.Duration) (time.Duration, string) {
	switch {
	case period < 2*time.Hour:
		return time.Hour, "hour"
	case period < 2*24*time.Hour:
		return 24 * time.Hour, "day"
	case period < 2*7*24*time.Hour:
		return 7 * 24 * time.Hour, "week"
	case period < 2*4*7*24*time.Hour:
		// We use 4 weeks, not 1 month
		return 4 * 7 * 24 * time.Hour, "month"
	default:
		return 0, "year"
	}
}

// previousPeriod shifts the provided input to the previous period and returns
// by how much it moved. The chosen period depend on the current period. For
// less than 2-hour period, the previous period is the hour. For less than 2-day
// period, this is the day. For less than 2-weeks, this is the week, for less
// than 2-months, this is the month, otherwise, this is the year. Also,
// dimensions are stripped.
func (input graphLineHandlerInput) previousPeriod() (graphLineHandlerInput, time.Duration) {
	input.Dimensions = []query.Column{}
	diff := input.End.Sub(input.Start)
	period, _ := nearestPeriod(diff)
	if period == 0 {
		// We use a full year this time (think for example we want to see how
		// was New Year Eve compared to last year). A year has no fixed length,
		// so it is measured from the start of the period. Both ends have to
		// move by the same amount, otherwise a leap day would give the previous
		// period one point more or less than the main one.
		period = input.Start.Sub(input.Start.AddDate(-1, 0, 0))
	}
	input.Start = input.Start.Add(-period)
	input.End = input.End.Add(-period)
	return input, period
}

type toSQL1Options struct {
	skipWithClause   bool
	reverseDirection bool
	// shiftedBy is how far back the range of this axis was moved. Its
	// timestamps move forward by the same amount, so the axis is drawn on the
	// time axis of the main period.
	shiftedBy time.Duration
}

func (input graphLineHandlerInput) toSQL1(axis int, r resolved, options toSQL1Options) *sb.Query {
	where := r.where(input.Filter)

	// The previous period is drawn on the time axis of the main one, so its
	// timestamps are shifted forward.
	shift := func(expr sb.Expr) sb.Expr {
		if options.shiftedBy == 0 {
			return expr
		}
		return sb.Op(expr, "+", seconds(uint64(options.shiftedBy.Seconds())))
	}

	// Units
	units := input.Units
	if options.reverseDirection {
		units = reverseUnits(units)
	}
	unitsSQL := unitsExpr(units)

	// Select
	selectFields := []sb.Expr{}
	dimensions := []sb.Expr{}
	others := []sb.Expr{}
	for _, column := range input.Dimensions {
		selectFields = append(selectFields, column.ToSQLSelect(input.schema, input.database))
		dimensions = append(dimensions, sb.Column(column.String()))
		others = append(others, sb.String("Other"))
	}
	// Rows outside the selected ones are folded into a single "Other" row. The
	// same value fills the gaps left by WITH FILL.
	othersArray := func() sb.Expr {
		if len(dimensions) == 0 {
			return sb.Function("emptyArrayString")
		}
		return sb.Array(others...)
	}
	dimensionsField := othersArray()
	if len(dimensions) > 0 {
		dimensionsField = sb.Function("if",
			sb.Op(sb.Tuple(dimensions...), "IN", sb.Column("rows")),
			sb.Array(selectFields...),
			othersArray())
	}

	inner := sb.Select(
		sb.Alias(shift(r.toStartOfInterval()), "time"),
		sb.Alias(sb.Op(unitsSQL, "/", sb.Uint(r.Interval)), "xps"),
		sb.Alias(dimensionsField, "dimensions"),
	).
		From(sb.Table("source")).
		Where(where).
		GroupBy(sb.Column("time"), sb.Column("dimensions")).
		OrderBy(sb.Order(sb.Column("time")).Fill(
			shift(r.timefilterStart()),
			shift(sb.Op(r.timefilterEnd(), "+", seconds(1))),
			sb.Uint(r.Interval))).
		Interpolate("dimensions", othersArray())

	query := sb.Select(
		sb.Alias(sb.Int(int64(axis)), "axis"),
		sb.Star()).
		FromSelect(inner)
	if !options.skipWithClause {
		query.With("source", input.sourceSelect(r.Table))
		if len(dimensions) > 0 {
			query.With("rows", selectLineRowsByLimitType(input, r, dimensions, where, unitsSQL))
		}
	}
	return query
}

// resolveContext returns what is needed to select the table for this query.
func (input graphLineHandlerInput) resolveContext() inputContext {
	return inputContext{
		Start:             input.Start,
		End:               input.End,
		MainTableRequired: requireMainTable(input.schema, input.Dimensions, input.Filter),
		Points:            input.Points,
	}
}

// toSQL converts a graph input to a list of SQL requests, one per axis.
func (input graphLineHandlerInput) toSQL(res resolution) []*sb.Query {
	// The resolution is applied once: the previous period reuses the resulting
	// range, moved back in time, so both cover the same number of points.
	main := res.forRange(input.Start, input.End)
	queries := []*sb.Query{input.toSQL1(1, main, toSQL1Options{})}
	if input.Bidirectional {
		queries = append(queries, input.reverseDirection().toSQL1(2, main, toSQL1Options{
			skipWithClause:   true,
			reverseDirection: true,
		}))
	}
	if input.PreviousPeriod {
		previous, period := input.previousPeriod()
		queries = append(queries, previous.toSQL1(3, main.shiftBack(period), toSQL1Options{
			skipWithClause: true,
			shiftedBy:      period,
		}))
	}
	if input.Bidirectional && input.PreviousPeriod {
		previous, period := input.reverseDirection().previousPeriod()
		queries = append(queries, previous.toSQL1(4, main.shiftBack(period), toSQL1Options{
			skipWithClause:   true,
			reverseDirection: true,
			shiftedBy:        period,
		}))
	}
	return queries
}

func (c *Component) graphLineHandlerFunc(w http.ResponseWriter, req *http.Request) {
	ctx := c.t.Context(req.Context())
	input := graphLineHandlerInput{
		schema:   c.d.Schema,
		database: c.d.ClickHouseDB.DatabaseName(),
	}
	if err := httpserver.BindJSON(req, &input); err != nil {
		httpserver.WriteJSON(w, http.StatusBadRequest, helpers.M{"message": helpers.Capitalize(err.Error())})
		return
	}
	if err := query.Columns(input.Dimensions).Validate(input.schema); err != nil {
		httpserver.WriteJSON(w, http.StatusBadRequest, helpers.M{"message": helpers.Capitalize(err.Error())})
		return
	}
	if err := input.Filter.Validate(input.schema, input.database); err != nil {
		httpserver.WriteJSON(w, http.StatusBadRequest, helpers.M{"message": helpers.Capitalize(err.Error())})
		return
	}
	if input.Limit > c.config.DimensionsLimit {
		httpserver.WriteJSON(w, http.StatusBadRequest,
			helpers.M{"message": fmt.Sprintf("Limit is set beyond maximum value (%d)",
				c.config.DimensionsLimit)})
		return
	}

	r := c.resolve(input.resolveContext())
	sqlQuery := unionAll(input.toSQL(r))
	w.Header().Set("X-SQL-Query", strings.ReplaceAll(sqlQuery, "\n", "  "))

	results := []struct {
		Axis       uint8     `ch:"axis"`
		Time       time.Time `ch:"time"`
		Xps        float64   `ch:"xps"`
		Dimensions []string  `ch:"dimensions"`
	}{}
	c.metrics.clickhouseQueries.WithLabelValues(r.Table).Inc()
	if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
		c.r.Err(err).Str("query", sqlQuery).Msg("unable to query database")
		httpserver.WriteJSON(w, http.StatusInternalServerError, helpers.M{"message": "Unable to query database."})
		return
	}

	// When requesting the previous period, we get an empty dimension in
	// results. Put it back.
	if len(input.Dimensions) > 0 {
		zeroDimensions := make([]string, len(input.Dimensions))
		for idx := range zeroDimensions {
			zeroDimensions[idx] = "Other"
		}
		for idx := range results {
			if len(results[idx].Dimensions) == 0 {
				results[idx].Dimensions = zeroDimensions
			}
		}
	}

	// Set time axis. We assume the first returned axis has the complete view.
	output := graphLineHandlerOutput{
		Time: []time.Time{},
	}
	lastTime := time.Time{}
	for _, result := range results {
		if result.Axis == 1 && !result.Time.Equal(lastTime) {
			output.Time = append(output.Time, result.Time)
			lastTime = result.Time
		}
	}

	// For the remaining, we will collect information into various
	// structures in one pass. Each structure will be keyed by the
	// axis and the row.
	axes := []int{}                       // list of axes
	rows := map[int]map[string][]string{} // for each axis, a map from row to list of dimensions
	points := map[int]map[string][]int{}  // for each axis, a map from row to list of points (one point per ts)
	sums := map[int]map[string]uint64{}   // for each axis, a map from row to sum (for sorting purpose)
	maxes := map[int]map[string]uint64{}  // for each axis, a map from row to max (for sorting purpose)
	lasts := map[int]map[string]int{}     // for each axis, a map from row to last(for sorting purpose)
	lastTimeForAxis := map[int]time.Time{}
	timeIndexForAxis := map[int]int{}
	for _, result := range results {
		var ok bool
		axis := int(result.Axis)
		lastTime, ok = lastTimeForAxis[axis]
		if !ok {
			// Unknown axis, initialize various structs
			axes = append(axes, axis)
			lastTimeForAxis[axis] = time.Time{}
			timeIndexForAxis[axis] = -1
			rows[axis] = map[string][]string{}
			points[axis] = map[string][]int{}
			sums[axis] = map[string]uint64{}
			maxes[axis] = map[string]uint64{}
			lasts[axis] = map[string]int{}
		}
		if !result.Time.Equal(lastTime) {
			// New timestamp, increment time index
			timeIndexForAxis[axis]++
			lastTimeForAxis[axis] = result.Time
		}
		rowKey := fmt.Sprintf("%d-%s", axis, result.Dimensions)
		_, ok = points[axis][rowKey]
		if !ok {
			// Not points for this row yet, create it
			rows[axis][rowKey] = result.Dimensions
			row := make([]int, len(output.Time))
			points[axis][rowKey] = row
			sums[axis][rowKey] = 0
			maxes[axis][rowKey] = 0
			lasts[axis][rowKey] = 0
		}
		points[axis][rowKey][timeIndexForAxis[axis]] = int(result.Xps)
		sums[axis][rowKey] += uint64(result.Xps)
		if uint64(result.Xps) > maxes[axis][rowKey] {
			maxes[axis][rowKey] = uint64(result.Xps)
		}
		lasts[axis][rowKey] = int(result.Xps)
	}
	// Sort axes
	sort.Ints(axes)
	// Sort the rows using the sums
	sortedRowKeys := map[int][]string{}
	for _, axis := range axes {
		sortedRowKeys[axis] = make([]string, 0, len(rows[axis]))
		for k := range rows[axis] {
			sortedRowKeys[axis] = append(sortedRowKeys[axis], k)
		}
		sort.Slice(sortedRowKeys[axis], func(i, j int) bool {
			iKey := sortedRowKeys[axis][i]
			jKey := sortedRowKeys[axis][j]
			if rows[axis][iKey][0] == "Other" {
				return false
			}
			if rows[axis][jKey][0] == "Other" {
				return true
			}
			if input.LimitType == "max" {
				return maxes[axis][iKey] > maxes[axis][jKey]
			}
			if input.LimitType == "last" {
				return lasts[axis][iKey] > lasts[axis][jKey]
			}

			return sums[axis][iKey] > sums[axis][jKey]
		})
	}

	// Now, we can complete the `output' structure!
	totalRows := 0
	for _, axis := range axes {
		totalRows += len(rows[axis])
	}

	// Compute total (not applicable for percentage units)
	computeTotal := !strings.HasSuffix(string(input.Units), "%")
	var intervalSeconds uint64
	if computeTotal && len(output.Time) >= 2 {
		intervalSeconds = uint64(output.Time[1].Sub(output.Time[0]).Seconds())
	}

	output.Rows = make([][]string, totalRows)
	output.Axis = make([]int, totalRows)
	output.AxisNames = make(map[int]string)
	output.Points = make([][]int, totalRows)
	output.Average = make([]int, totalRows)
	if computeTotal {
		output.Total = make([]int, totalRows)
	}
	output.Min = make([]int, totalRows)
	output.Max = make([]int, totalRows)
	output.Last = make([]int, totalRows)
	output.NinetyFivePercentile = make([]int, totalRows)

	i := -1
	for _, axis := range axes {
		for _, k := range sortedRowKeys[axis] {
			i++
			output.Rows[i] = rows[axis][k]
			output.Axis[i] = axis
			output.Points[i] = points[axis][k]
			output.Average[i] = int(sums[axis][k] / uint64(len(output.Time)))
			if computeTotal {
				output.Total[i] = int(sums[axis][k] * intervalSeconds)
			}

			// Last
			// We use the second last value (-2) because
			// the last value is not show in the graph.
			output.Last[i] = points[axis][k][len(output.Time)-2]

			// For remaining, we will sort the values. It
			// is needed for 95th percentile but it helps
			// for min/max too. We remove special cases
			// for 0 or 1 point.
			nbPoints := len(output.Points[i])
			if nbPoints == 0 {
				continue
			}
			if nbPoints == 1 {
				v := output.Points[i][0]
				output.Min[i] = v
				output.Max[i] = v
				output.Last[i] = v
				output.NinetyFivePercentile[i] = v
				continue
			}

			points := make([]int, nbPoints)
			copy(points, output.Points[i])
			sort.Ints(points)

			// Min (but not 0)
			for j := range nbPoints {
				output.Min[i] = points[j]
				if points[j] > 0 {
					break
				}
			}
			// Max
			output.Max[i] = points[nbPoints-1]
			// 95th percentile with linear interpolation
			index := 0.95 * float64(nbPoints-1)
			j := int(index)
			fraction := index - float64(j)
			output.NinetyFivePercentile[i] = points[nbPoints-1]
			if j >= 0 && j < nbPoints-1 {
				output.NinetyFivePercentile[i] = int(
					math.Round(float64(points[j])*(1-fraction) + float64(points[j+1])*fraction))
			}
		}
	}

	for _, axis := range output.Axis {
		switch axis {
		case 1:
			output.AxisNames[axis] = "Direct"
		case 2:
			output.AxisNames[axis] = "Reverse"
		case 3, 4:
			diff := input.End.Sub(input.Start)
			_, name := nearestPeriod(diff)
			output.AxisNames[axis] = fmt.Sprintf("Previous %s", name)
		}
	}
	httpserver.WriteJSON(w, http.StatusOK, output)
}

type tableIntervalInput struct {
	Start  time.Time `json:"start" validate:"required"`
	End    time.Time `json:"end" validate:"required,gtfield=Start"`
	Points uint      `json:"points" validate:"required,min=5,max=2000"` // minimum number of points
}

type tableIntervalOutput struct {
	Table    string `json:"table"`
	Interval uint64 `json:"interval"`
}

func (c *Component) getTableAndIntervalHandlerFunc(w http.ResponseWriter, req *http.Request) {
	var input tableIntervalInput
	if err := httpserver.BindJSON(req, &input); err != nil {
		httpserver.WriteJSON(w, http.StatusBadRequest, helpers.M{"message": helpers.Capitalize(err.Error())})
		return
	}
	table, interval, _ := c.computeTableAndInterval(inputContext{
		Points: input.Points,
		Start:  input.Start,
		End:    input.End,
	})

	httpserver.WriteJSON(w, http.StatusOK, tableIntervalOutput{Table: table, Interval: uint64(interval.Seconds())})
}
