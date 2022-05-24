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

// sankeyQuery describes the input for the /sankey endpoint.
type sankeyQuery struct {
	Start      time.Time     `json:"start" binding:"required"`
	End        time.Time     `json:"end" binding:"required,gtfield=Start"`
	Dimensions []queryColumn `json:"dimensions" binding:"required,min=2"` // group by ...
	Limit      int           `json:"limit" binding:"min=1,max=50"`        // limit product of dimensions
	Filter     queryFilter   `json:"filter"`                              // where ...
	Units      string        `json:"units" binding:"required,oneof=pps bps"`
}

// sankeyQueryToSQL converts a sankey query to an SQL request
func (query sankeyQuery) toSQL() (string, error) {
	// Filter
	where := query.Filter.filter
	if where == "" {
		where = "{timefilter}"
	} else {
		where = fmt.Sprintf("{timefilter} AND (%s)", where)
	}

	// Select
	arrayFields := []string{}
	dimensions := []string{}
	for _, column := range query.Dimensions {
		arrayFields = append(arrayFields, fmt.Sprintf(`if(%s IN (SELECT %s FROM rows), %s, 'Other')`,
			column.String(),
			column.String(),
			column.toSQLSelect()))
		dimensions = append(dimensions, column.String())
	}
	fields := []string{}
	if query.Units == "pps" {
		fields = append(fields, `SUM(Packets*SamplingRate/range) AS xps`)
	} else {
		fields = append(fields, `SUM(Bytes*SamplingRate*8/range) AS xps`)
	}
	fields = append(fields, fmt.Sprintf("[%s] AS dimensions", strings.Join(arrayFields, ",\n  ")))

	// With
	with := []string{
		fmt.Sprintf(`(SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM {table} WHERE %s) AS range`, where),
		fmt.Sprintf(
			"rows AS (SELECT %s FROM {table} WHERE %s GROUP BY %s ORDER BY SUM(Bytes) DESC LIMIT %d)",
			strings.Join(dimensions, ", "),
			where,
			strings.Join(dimensions, ", "),
			query.Limit),
	}

	sqlQuery := fmt.Sprintf(`
WITH
 %s
SELECT
 %s
FROM {table}
WHERE %s
GROUP BY dimensions
ORDER BY xps DESC`, strings.Join(with, ",\n "), strings.Join(fields, ",\n "), where)
	return sqlQuery, nil
}

type sankeyHandlerOutput struct {
	// Unprocessed data for table view
	Rows [][]string `json:"rows"`
	Xps  []int      `json:"xps"` // row → xps
	// Processed data for sankey graph
	Nodes []string     `json:"nodes"`
	Links []sankeyLink `json:"links"`
}
type sankeyLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Xps    int    `json:"xps"`
}

func (c *Component) sankeyHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	var query sankeyQuery
	if err := gc.ShouldBindJSON(&query); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}

	sqlQuery, err := query.toSQL()
	if err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}

	// We need to select a resolution allowing us to have a somewhat accurate timespan
	resolution := time.Duration(int64(query.End.Sub(query.Start).Nanoseconds()) / 20)
	if resolution < time.Second {
		resolution = time.Second
	}

	// Prepare and execute query
	sqlQuery = c.queryFlowsTable(sqlQuery,
		query.Start, query.End, resolution)
	gc.Header("X-SQL-Query", strings.ReplaceAll(sqlQuery, "\n", "  "))
	results := []struct {
		Xps        float64  `ch:"xps"`
		Dimensions []string `ch:"dimensions"`
	}{}
	if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
		c.r.Err(err).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}

	// Prepare output
	output := sankeyHandlerOutput{
		Rows:  make([][]string, 0, len(results)),
		Xps:   make([]int, 0, len(results)),
		Nodes: make([]string, 0),
		Links: make([]sankeyLink, 0),
	}
	completeName := func(name string, index int) string {
		if name != "Other" {
			return name
		}
		return fmt.Sprintf("Other %s", query.Dimensions[index].String())
	}
	addedNodes := map[string]bool{}
	addNode := func(name string) {
		if _, ok := addedNodes[name]; !ok {
			addedNodes[name] = true
			output.Nodes = append(output.Nodes, name)
		}
	}
	addLink := func(source, target string, xps int) {
		for idx, link := range output.Links {
			if link.Source == source && link.Target == target {
				output.Links[idx].Xps += xps
				return
			}
		}
		output.Links = append(output.Links, sankeyLink{source, target, xps})
	}
	for _, result := range results {
		output.Rows = append(output.Rows, result.Dimensions)
		output.Xps = append(output.Xps, int(result.Xps))
		// Consider each pair of successive dimensions
		for i := 0; i < len(query.Dimensions)-1; i++ {
			dimension1 := completeName(result.Dimensions[i], i)
			dimension2 := completeName(result.Dimensions[i+1], i+1)
			addNode(dimension1)
			addNode(dimension2)
			addLink(dimension1, dimension2, int(result.Xps))
		}
	}
	sort.Slice(output.Links, func(i, j int) bool {
		if output.Links[i].Xps == output.Links[j].Xps {
			return output.Links[i].Source < output.Links[j].Source
		}
		return output.Links[i].Xps > output.Links[j].Xps
	})

	gc.JSON(http.StatusOK, output)
}
