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

// graphQuery describes the input for the /graph endpoint.
type graphQuery struct {
	Start      time.Time     `json:"start" binding:"required"`
	End        time.Time     `json:"end" binding:"required"`
	Points     int           `json:"points" binding:"required"` // minimum number of points
	Dimensions []queryColumn `json:"dimensions"`                // group by ...
	Limit      int           `json:"limit"`                     // limit product of dimensions
	Filter     queryFilter   `json:"filter"`                    // where ...
}

// graphQueryToSQL converts a graph query to an SQL request
func (query graphQuery) toSQL() (string, error) {
	interval := int64((query.End.Sub(query.Start).Seconds())) / int64(query.Points)

	// Filter
	where := query.Filter.filter
	if where == "" {
		where = "{timefilter}"
	} else {
		where = fmt.Sprintf("{timefilter} AND (%s)", where)
	}

	// Select
	fields := []string{
		`toStartOfInterval(TimeReceived, INTERVAL slot second) AS time`,
		`SUM(Bytes*SamplingRate*8/slot) AS bps`,
	}
	selectFields := []string{}
	dimensions := []string{}
	others := []string{}
	for _, column := range query.Dimensions {
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
			query.Limit))
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

type graphHandlerOutput struct {
	Rows    [][]string  `json:"rows"`
	Time    []time.Time `json:"t"`
	Points  [][]int     `json:"points"`  // t → row → bps
	Average []int       `json:"average"` // row → bps
	Min     []int       `json:"min"`
	Max     []int       `json:"max"`
}

func (c *Component) graphHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	var query graphQuery
	if err := gc.ShouldBindJSON(&query); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}
	if query.Start.After(query.End) {
		gc.JSON(http.StatusBadRequest, gin.H{"message": "Start should not be after end"})
		return
	}
	if query.Points < 5 || query.Points > 2000 {
		gc.JSON(http.StatusBadRequest, gin.H{"message": "Points should be >= 5 and <= 2000"})
		return
	}
	if query.Limit == 0 {
		query.Limit = 10
	}
	if query.Limit < 5 || query.Limit > 50 {
		gc.JSON(http.StatusBadRequest, gin.H{"message": "Limit should be >= 5 and <= 50"})
		return
	}

	sqlQuery, err := query.toSQL()
	if err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}
	resolution := time.Duration(int64(query.End.Sub(query.Start).Nanoseconds()) / int64(query.Points))
	if resolution < time.Second {
		resolution = time.Second
	}
	sqlQuery = c.queryFlowsTable(sqlQuery,
		query.Start, query.End, resolution)
	gc.Header("X-SQL-Query", sqlQuery)

	results := []struct {
		Time       time.Time `ch:"time"`
		Bps        float64   `ch:"bps"`
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
		rowValues[rowKey][idx] = int(result.Bps)
		sum, _ := rowSums[rowKey]
		rowSums[rowKey] = sum + uint64(result.Bps)
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

	for idx, r := range rows {
		output.Rows[idx] = rowKeys[r]
		output.Points[idx] = rowValues[r]
		output.Average[idx] = int(rowSums[r] / uint64(len(output.Time)))
		for j, v := range rowValues[r] {
			if j == 0 || output.Min[idx] > v {
				output.Min[idx] = v
			}
			if j == 0 || output.Max[idx] < v {
				output.Max[idx] = v
			}
		}
	}

	gc.JSON(http.StatusOK, output)
}
