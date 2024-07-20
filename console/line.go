// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/exp/slices"

	"akvorado/common/helpers"
	"akvorado/console/query"
)

// graphLineHandlerInput describes the input for the /graph/line endpoint.
type graphLineHandlerInput struct {
	graphCommonHandlerInput
	Points         uint `json:"points" binding:"required,min=5,max=2000"` // minimum number of points
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
	Average              []int          `json:"average"` // row → average xps
	Min                  []int          `json:"min"`     // row → min xps
	Max                  []int          `json:"max"`     // row → max xps
	NinetyFivePercentile []int          `json:"95th"`    // row → 95th xps
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

// previousPeriod shifts the provided input to the previous period.
// The chosen period depend on the current period. For less than
// 2-hour period, the previous period is the hour. For less than 2-day
// period, this is the day. For less than 2-weeks, this is the week,
// for less than 2-months, this is the month, otherwise, this is the
// year. Also, dimensions are stripped.
func (input graphLineHandlerInput) previousPeriod() graphLineHandlerInput {
	input.Dimensions = []query.Column{}
	diff := input.End.Sub(input.Start)
	period, _ := nearestPeriod(diff)
	if period == 0 {
		// We use a full year this time (think for example we
		// want to see how was New Year Eve compared to last
		// year)
		input.Start = input.Start.AddDate(-1, 0, 0)
		input.End = input.End.AddDate(-1, 0, 0)
		return input
	}
	input.Start = input.Start.Add(-period)
	input.End = input.End.Add(-period)
	return input
}

type toSQL1Options struct {
	skipWithClause   bool
	reverseDirection bool
	offsetedStart    time.Time
}

func (input graphLineHandlerInput) toSQL1(axis int, options toSQL1Options) string {
	var startForInterval *time.Time
	var offsetShift string
	if !options.offsetedStart.IsZero() {
		startForInterval = &options.offsetedStart
		offsetShift = fmt.Sprintf(" + INTERVAL %d second",
			int64(options.offsetedStart.Sub(input.Start).Seconds()))
	}
	where := templateWhere(input.Filter)

	// Select
	fields := []string{
		fmt.Sprintf(`{{ call .ToStartOfInterval "TimeReceived" }}%s AS time`, offsetShift),
		`{{ .Units }}/{{ .Interval }} AS xps`,
	}
	selectFields := []string{}
	dimensions := []string{}
	dimensionsInterpolate := ""
	others := []string{}
	for _, column := range input.Dimensions {
		field := column.ToSQLSelect(input.schema)
		selectFields = append(selectFields, field)
		dimensions = append(dimensions, column.String())
		others = append(others, "'Other'")
	}
	if len(dimensions) > 0 {
		fields = append(fields, fmt.Sprintf(`if((%s) IN rows, [%s], [%s]) AS dimensions`,
			strings.Join(dimensions, ", "),
			strings.Join(selectFields, ", "),
			strings.Join(others, ", ")))
		dimensionsInterpolate = fmt.Sprintf("[%s]", strings.Join(others, ", "))
	} else {
		fields = append(fields, "emptyArrayString() AS dimensions")
		dimensionsInterpolate = "emptyArrayString()"
	}

	// With
	withStr := ""
	if !options.skipWithClause {
		with := []string{fmt.Sprintf("source AS (%s)", input.sourceSelect())}
		if len(dimensions) > 0 {
			with = append(with, fmt.Sprintf(
				"rows AS (SELECT %s FROM source WHERE %s GROUP BY %s ORDER BY SUM(%s) DESC LIMIT %d)",
				strings.Join(dimensions, ", "),
				where,
				strings.Join(dimensions, ", "),
				metricForTopSort(input.Units),
				input.Limit))
		}
		if len(with) > 0 {
			withStr = fmt.Sprintf("\nWITH\n %s", strings.Join(with, ",\n "))
		}
	}

	// Units
	units := input.Units
	if options.reverseDirection {
		switch units {
		case "inl2%":
			units = "outl2%"
		case "outl2%":
			units = "inl2%"
		}
	}

	sqlQuery := fmt.Sprintf(`
{{ with %s }}%s
SELECT %d AS axis, * FROM (
SELECT
 %s
FROM source
WHERE %s
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}%s
 TO {{ .TimefilterEnd }} + INTERVAL 1 second%s
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS %s))
{{ end }}`,
		templateContext(inputContext{
			Start:             input.Start,
			End:               input.End,
			StartForInterval:  startForInterval,
			MainTableRequired: requireMainTable(input.schema, input.Dimensions, input.Filter),
			Points:            input.Points,
			Units:             units,
		}),
		withStr, axis, strings.Join(fields, ",\n "), where, offsetShift, offsetShift,
		dimensionsInterpolate,
	)
	return strings.TrimSpace(sqlQuery)
}

// toSQL converts a graph input to an SQL request
func (input graphLineHandlerInput) toSQL() string {
	parts := []string{input.toSQL1(1, toSQL1Options{})}
	// Handle specific options. We have to align time periods in
	// case the previous period does not use the same offsets.
	if input.Bidirectional {
		parts = append(parts, input.reverseDirection().toSQL1(2, toSQL1Options{
			skipWithClause:   true,
			reverseDirection: true,
		}))
	}
	if input.PreviousPeriod {
		parts = append(parts, input.previousPeriod().toSQL1(3, toSQL1Options{
			skipWithClause: true,
			offsetedStart:  input.Start,
		}))
	}
	if input.Bidirectional && input.PreviousPeriod {
		parts = append(parts, input.reverseDirection().previousPeriod().toSQL1(4, toSQL1Options{
			skipWithClause:   true,
			reverseDirection: true,
			offsetedStart:    input.Start,
		}))
	}
	return strings.Join(parts, "\nUNION ALL\n")
}

func (c *Component) graphLineHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	input := graphLineHandlerInput{graphCommonHandlerInput: graphCommonHandlerInput{schema: c.d.Schema}}
	if err := gc.ShouldBindJSON(&input); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}
	if err := query.Columns(input.Dimensions).Validate(input.schema); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}
	if err := input.Filter.Validate(input.schema); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}
	if input.Limit > c.config.DimensionsLimit {
		gc.JSON(http.StatusBadRequest,
			gin.H{"message": fmt.Sprintf("Limit is set beyond maximum value (%d)",
				c.config.DimensionsLimit)})
		return
	}

	sqlQuery := input.toSQL()
	sqlQuery = c.finalizeQuery(sqlQuery)
	gc.Header("X-SQL-Query", strings.ReplaceAll(sqlQuery, "\n", "  "))

	results := []struct {
		Axis       uint8     `ch:"axis"`
		Time       time.Time `ch:"time"`
		Xps        float64   `ch:"xps"`
		Dimensions []string  `ch:"dimensions"`
	}{}
	if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
		c.r.Err(err).Str("query", sqlQuery).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}

	// When filling 0 value, we may get an empty dimensions.
	// From ClickHouse 22.4, it is possible to do interpolation database-side
	// (INTERPOLATE (['Other', 'Other'] AS Dimensions))
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
		if result.Axis == 1 && result.Time != lastTime {
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
		}
		if result.Time != lastTime {
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
		}
		points[axis][rowKey][timeIndexForAxis[axis]] = int(result.Xps)
		sums[axis][rowKey] += uint64(result.Xps)
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
			return sums[axis][iKey] > sums[axis][jKey]
		})
	}

	// Now, we can complete the `output' structure!
	totalRows := 0
	for _, axis := range axes {
		totalRows += len(rows[axis])
	}
	output.Rows = make([][]string, totalRows)
	output.Axis = make([]int, totalRows)
	output.AxisNames = make(map[int]string)
	output.Points = make([][]int, totalRows)
	output.Average = make([]int, totalRows)
	output.Min = make([]int, totalRows)
	output.Max = make([]int, totalRows)
	output.NinetyFivePercentile = make([]int, totalRows)

	i := -1
	for _, axis := range axes {
		for _, k := range sortedRowKeys[axis] {
			i++
			output.Rows[i] = rows[axis][k]
			output.Axis[i] = axis
			output.Points[i] = points[axis][k]
			output.Average[i] = int(sums[axis][k] / uint64(len(output.Time)))

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
				output.NinetyFivePercentile[i] = v
				continue
			}

			points := make([]int, nbPoints)
			copy(points, output.Points[i])
			sort.Ints(points)

			// Min (but not 0)
			for j := 0; j < nbPoints; j++ {
				output.Min[i] = points[j]
				if points[j] > 0 {
					break
				}
			}
			// Max
			output.Max[i] = points[nbPoints-1]
			// 95th percentile
			index := 0.95 * float64(nbPoints)
			j := int(index)
			if index == float64(j) {
				output.NinetyFivePercentile[i] = points[j-1]
			} else if index > 1 {
				// We use the average of the two values. This
				// is good enough for bps/pps
				output.NinetyFivePercentile[i] = (points[j-1] + points[j]) / 2
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
	gc.JSON(http.StatusOK, output)
}

type tableIntervalInput struct {
	Start  time.Time `json:"start" binding:"required"`
	End    time.Time `json:"end" binding:"required,gtfield=Start"`
	Points uint      `json:"points" binding:"required,min=5,max=2000"` // minimum number of points
}

type tableIntervalOutput struct {
	Table    string `json:"table"`
	Interval uint64 `json:"interval"`
}

func (c *Component) getTableAndIntervalHandlerFunc(gc *gin.Context) {
	var input tableIntervalInput
	if err := gc.ShouldBindJSON(&input); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}
	table, interval, _ := c.computeTableAndInterval(inputContext{
		Points: input.Points,
		Start:  input.Start,
		End:    input.End,
	})

	gc.JSON(http.StatusOK, tableIntervalOutput{Table: table, Interval: uint64(interval.Seconds())})
}
