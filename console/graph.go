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
	Start      time.Time     `json:"start" binding:"required"`
	End        time.Time     `json:"end" binding:"required,gtfield=Start"`
	Points     int           `json:"points" binding:"required,min=5,max=2000"` // minimum number of points
	Dimensions []queryColumn `json:"dimensions"`                               // group by ...
	Limit      int           `json:"limit" binding:"min=1,max=50"`             // limit product of dimensions
	Filter     queryFilter   `json:"filter"`                                   // where ...
	Units      string        `json:"units" binding:"required,oneof=pps l2bps l3bps"`
}

// graphHandlerOutput describes the output for the /graph endpoint.
type graphHandlerOutput struct {
	Rows                 [][]string  `json:"rows"`
	Time                 []time.Time `json:"t"`
	Points               [][]int     `json:"points"`  // t → row → xps
	Average              []int       `json:"average"` // row → xps
	Min                  []int       `json:"min"`
	Max                  []int       `json:"max"`
	NinetyFivePercentile []int       `json:"95th"`
}

// graphHandlerInputToSQL converts a graph input to an SQL request
func (input graphHandlerInput) toSQL() (string, error) {
	interval := int64((input.End.Sub(input.Start).Seconds())) / int64(input.Points)

	// Filter
	where := input.Filter.filter
	if where == "" {
		where = "{timefilter}"
	} else {
		where = fmt.Sprintf("{timefilter} AND (%s)", where)
	}

	// Select
	fields := []string{
		`toStartOfInterval(TimeReceived, INTERVAL slot second) AS time`,
	}
	switch input.Units {
	case "pps":
		fields = append(fields, `SUM(Packets*SamplingRate/slot) AS xps`)
	case "l3bps":
		fields = append(fields, `SUM(Bytes*SamplingRate*8/slot) AS xps`)
	case "l2bps":
		fields = append(fields, `SUM((Bytes+18*Packets)*SamplingRate*8/slot) AS xps`)
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
	with := []string{fmt.Sprintf(`intDiv(%d, {resolution})*{resolution} AS slot`, interval)}
	if len(dimensions) > 0 {
		with = append(with, fmt.Sprintf(
			"rows AS (SELECT %s FROM {table} WHERE %s GROUP BY %s ORDER BY SUM(Bytes) DESC LIMIT %d)",
			strings.Join(dimensions, ", "),
			where,
			strings.Join(dimensions, ", "),
			input.Limit))
	}

	sqlQuery := fmt.Sprintf(`
WITH
 %s
SELECT
 %s
FROM {table}
WHERE %s
GROUP BY time, dimensions
ORDER BY time`, strings.Join(with, ",\n "), strings.Join(fields, ",\n "), where)
	return sqlQuery, nil
}

func (c *Component) graphHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	var input graphHandlerInput
	if err := gc.ShouldBindJSON(&input); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}

	sqlQuery, err := input.toSQL()
	if err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}
	resolution := time.Duration(int64(input.End.Sub(input.Start).Nanoseconds()) / int64(input.Points))
	if resolution < time.Second {
		resolution = time.Second
	}
	sqlQuery = c.queryFlowsTable(sqlQuery,
		input.Start, input.End, resolution)
	gc.Header("X-SQL-Query", strings.ReplaceAll(sqlQuery, "\n", "  "))

	results := []struct {
		Time       time.Time `ch:"time"`
		Xps        float64   `ch:"xps"`
		Dimensions []string  `ch:"dimensions"`
	}{}
	if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
		c.r.Err(err).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}

	// We want to sort rows depending on how much data they gather each
	output := graphHandlerOutput{
		Time: []time.Time{},
	}
	rowValues := map[string][]int{}  // values for each row (indexed by internal key)
	rowKeys := map[string][]string{} // mapping from keys to dimensions
	rowSums := map[string]uint64{}   // sum for a given row (to sort)
	lastTime := time.Time{}
	for _, result := range results {
		if result.Time != lastTime {
			output.Time = append(output.Time, result.Time)
			lastTime = result.Time
		}
	}
	lastTime = time.Time{}
	idx := -1
	for _, result := range results {
		if result.Time != lastTime {
			idx++
			lastTime = result.Time
		}
		rowKey := fmt.Sprintf("%s", result.Dimensions)
		row, ok := rowValues[rowKey]
		if !ok {
			rowKeys[rowKey] = result.Dimensions
			row = make([]int, len(output.Time))
			rowValues[rowKey] = row
		}
		rowValues[rowKey][idx] = int(result.Xps)
		sum, _ := rowSums[rowKey]
		rowSums[rowKey] = sum + uint64(result.Xps)
	}
	rows := make([]string, len(rowKeys))
	i := 0
	for k := range rowKeys {
		rows[i] = k
		i++
	}
	// Sort by sum, except we want "Other" to be last
	sort.Slice(rows, func(i, j int) bool {
		if rowKeys[rows[i]][0] == "Other" {
			return false
		}
		if rowKeys[rows[j]][0] == "Other" {
			return true
		}
		return rowSums[rows[i]] > rowSums[rows[j]]
	})
	output.Rows = make([][]string, len(rows))
	output.Points = make([][]int, len(rows))
	output.Average = make([]int, len(rows))
	output.Min = make([]int, len(rows))
	output.Max = make([]int, len(rows))
	output.NinetyFivePercentile = make([]int, len(rows))

	for idx, r := range rows {
		output.Rows[idx] = rowKeys[r]
		output.Points[idx] = rowValues[r]
		output.Average[idx] = int(rowSums[r] / uint64(len(output.Time)))
		// For 95th percentile, we need to sort the values.
		// Use that for min/max too.
		if len(rowValues[r]) == 0 {
			continue
		}
		if len(rowValues[r]) == 1 {
			v := rowValues[r][0]
			output.Min[idx] = v
			output.Max[idx] = v
			output.NinetyFivePercentile[idx] = v
			continue
		}

		s := make([]int, len(rowValues[r]))
		copy(s, rowValues[r])
		sort.Ints(s)
		output.Min[idx] = s[0]
		output.Max[idx] = s[len(s)-1]
		index := 0.95 * float64(len(s))
		j := int(index)
		if index == float64(j) {
			output.NinetyFivePercentile[idx] = s[j-1]
		} else if index > 1 {
			// We use the average of the two values. This
			// is good enough for bps/pps
			output.NinetyFivePercentile[idx] = (s[j-1] + s[j]) / 2
		}
	}

	gc.JSON(http.StatusOK, output)
}
