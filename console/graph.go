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

	"akvorado/common/helpers"
)

// graphHandlerInput describes the input for the /graph endpoint.
type graphHandlerInput struct {
	Start         time.Time     `json:"start" binding:"required"`
	End           time.Time     `json:"end" binding:"required,gtfield=Start"`
	Points        int           `json:"points" binding:"required,min=5,max=2000"` // minimum number of points
	Dimensions    []queryColumn `json:"dimensions"`                               // group by ...
	Limit         int           `json:"limit" binding:"min=1,max=50"`             // limit product of dimensions
	Filter        queryFilter   `json:"filter"`                                   // where ...
	Units         string        `json:"units" binding:"required,oneof=pps l2bps l3bps"`
	Bidirectional bool          `json:"bidirectional"`
}

// graphHandlerOutput describes the output for the /graph endpoint. A
// row is a set of values for dimensions. Currently, axis 1 is for the
// direct direction and axis 2 is for the reverse direction. Rows are
// sorted by axis, then by the sum of traffic.
type graphHandlerOutput struct {
	Time                 []time.Time `json:"t"`
	Rows                 [][]string  `json:"rows"`    // List of rows
	Points               [][]int     `json:"points"`  // t → row → xps
	Axis                 []int       `json:"axis"`    // row → axis
	Average              []int       `json:"average"` // row → average xps
	Min                  []int       `json:"min"`     // row → min xps
	Max                  []int       `json:"max"`     // row → max xps
	NinetyFivePercentile []int       `json:"95th"`    // row → 95th xps
}

// reverseDirection reverts the direction of a provided input
func (input graphHandlerInput) reverseDirection() graphHandlerInput {
	input.Filter.Filter, input.Filter.ReverseFilter = input.Filter.ReverseFilter, input.Filter.Filter
	dimensions := input.Dimensions
	input.Dimensions = make([]queryColumn, len(dimensions))
	for i := range dimensions {
		input.Dimensions[i] = dimensions[i].reverseDirection()
	}
	return input
}

func (input graphHandlerInput) toSQL1(axis int, skipWith bool) string {
	interval := int64((input.End.Sub(input.Start).Seconds())) / int64(input.Points)
	slot := fmt.Sprintf(`{resolution->%d}`, interval)

	// Filter
	where := input.Filter.Filter
	if where == "" {
		where = "{timefilter}"
	} else {
		where = fmt.Sprintf("{timefilter} AND (%s)", where)
	}

	// Select
	fields := []string{
		fmt.Sprintf(`toStartOfInterval(TimeReceived, INTERVAL %s second) AS time`, slot),
	}
	switch input.Units {
	case "pps":
		fields = append(fields, fmt.Sprintf(`SUM(Packets*SamplingRate/%s) AS xps`, slot))
	case "l3bps":
		fields = append(fields, fmt.Sprintf(`SUM(Bytes*SamplingRate*8/%s) AS xps`, slot))
	case "l2bps":
		fields = append(fields, fmt.Sprintf(`SUM((Bytes+18*Packets)*SamplingRate*8/%s) AS xps`, slot))
	}
	selectFields := []string{}
	dimensions := []string{}
	others := []string{}
	for _, column := range input.Dimensions {
		field := column.toSQLSelect()
		selectFields = append(selectFields, field)
		dimensions = append(dimensions, column.String())
		others = append(others, "'Other'")
	}
	if len(dimensions) > 0 {
		fields = append(fields, fmt.Sprintf(`if((%s) IN rows, [%s], [%s]) AS dimensions`,
			strings.Join(dimensions, ", "),
			strings.Join(selectFields, ", "),
			strings.Join(others, ", ")))
	} else {
		fields = append(fields, "emptyArrayString() AS dimensions")
	}

	// With
	with := []string{}
	if len(dimensions) > 0 && !skipWith {
		with = append(with, fmt.Sprintf(
			"rows AS (SELECT %s FROM {table} WHERE %s GROUP BY %s ORDER BY SUM(Bytes) DESC LIMIT %d)",
			strings.Join(dimensions, ", "),
			where,
			strings.Join(dimensions, ", "),
			input.Limit))
	}
	withStr := ""
	if len(with) > 0 {
		withStr = fmt.Sprintf("WITH\n %s", strings.Join(with, ",\n "))
	}

	sqlQuery := fmt.Sprintf(`
%s
SELECT %d AS axis, * FROM (
SELECT
 %s
FROM {table}
WHERE %s
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM toStartOfInterval({timefilter.Start}, INTERVAL %s second)
 TO {timefilter.Stop}
 STEP %s)`, withStr, axis, strings.Join(fields, ",\n "), where, slot, slot)
	return sqlQuery
}

// graphHandlerInputToSQL converts a graph input to an SQL request
func (input graphHandlerInput) toSQL() string {
	result := input.toSQL1(1, false)
	if input.Bidirectional {
		part2 := input.reverseDirection().toSQL1(2, true)
		result = fmt.Sprintf(`%s
UNION ALL
%s`, result, strings.TrimSpace(part2))
	}
	return strings.TrimSpace(result)
}

func (c *Component) graphHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	var input graphHandlerInput
	if err := gc.ShouldBindJSON(&input); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}

	sqlQuery := input.toSQL()
	resolution := time.Duration(int64(input.End.Sub(input.Start).Nanoseconds()) / int64(input.Points))
	if resolution < time.Second {
		resolution = time.Second
	}
	sqlQuery = c.queryFlowsTable(sqlQuery, input.Filter.MainTableRequired,
		input.Start, input.End, resolution)
	gc.Header("X-SQL-Query", strings.ReplaceAll(sqlQuery, "\n", "  "))

	results := []struct {
		Axis       uint8     `ch:"axis"`
		Time       time.Time `ch:"time"`
		Xps        float64   `ch:"xps"`
		Dimensions []string  `ch:"dimensions"`
	}{}
	if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
		c.r.Err(err).Msg("unable to query database")
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
	output := graphHandlerOutput{
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
		row, ok := points[axis][rowKey]
		if !ok {
			// Not points for this row yet, create it
			rows[axis][rowKey] = result.Dimensions
			row = make([]int, len(output.Time))
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

	gc.JSON(http.StatusOK, output)
}
