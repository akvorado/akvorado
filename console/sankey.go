// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
	"akvorado/console/query"
)

// graphSankeyHandlerInput describes the input for the /graph/sankey endpoint.
type graphSankeyHandlerInput struct {
	graphCommonHandlerInput
}

// graphSankeyHandlerOutput describes the output for the /graph/sankey endpoint.
type graphSankeyHandlerOutput struct {
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

// sankeyHandlerInputToSQL converts a sankey query to an SQL request
func (input graphSankeyHandlerInput) toSQL() (string, error) {
	where := templateWhere(input.Filter)

	// Select
	arrayFields := []string{}
	dimensions := []string{}
	for _, column := range input.Dimensions {
		arrayFields = append(arrayFields, fmt.Sprintf(`if(%s IN (SELECT %s FROM rows), %s, 'Other')`,
			column.String(),
			column.String(),
			column.ToSQLSelect(input.schema)))
		dimensions = append(dimensions, column.String())
	}
	fields := []string{
		`{{ .Units }}/range AS xps`,
		fmt.Sprintf("[%s] AS dimensions", strings.Join(arrayFields, ",\n  ")),
	}

	// With
	with := []string{
		fmt.Sprintf("source AS (%s)", input.sourceSelect()),
		fmt.Sprintf(`(SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM source WHERE %s) AS range`, where),
		fmt.Sprintf(
			"rows AS (SELECT %s FROM source WHERE %s GROUP BY %s ORDER BY SUM(%s) DESC LIMIT %d)",
			strings.Join(dimensions, ", "),
			where,
			strings.Join(dimensions, ", "),
			metricForTopSort(input.Units),
			input.Limit),
	}

	sqlQuery := fmt.Sprintf(`
{{ with %s }}
WITH
 %s
SELECT
 %s
FROM source
WHERE %s
GROUP BY dimensions
ORDER BY xps DESC
{{ end }}`,
		templateContext(inputContext{
			Start:             input.Start,
			End:               input.End,
			MainTableRequired: requireMainTable(input.schema, input.Dimensions, input.Filter),
			Points:            20,
			Units:             input.Units,
		}),
		strings.Join(with, ",\n "), strings.Join(fields, ",\n "), where)
	return strings.TrimSpace(sqlQuery), nil
}

func (c *Component) graphSankeyHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	input := graphSankeyHandlerInput{graphCommonHandlerInput: graphCommonHandlerInput{schema: c.d.Schema}}
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

	sqlQuery, err := input.toSQL()
	if err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}

	// Prepare and execute query
	sqlQuery = c.finalizeQuery(sqlQuery)
	gc.Header("X-SQL-Query", strings.ReplaceAll(sqlQuery, "\n", "  "))
	results := []struct {
		Xps        float64  `ch:"xps"`
		Dimensions []string `ch:"dimensions"`
	}{}
	if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
		c.r.Err(err).Str("query", sqlQuery).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}

	// Prepare output
	output := graphSankeyHandlerOutput{
		Rows:  make([][]string, 0, len(results)),
		Xps:   make([]int, 0, len(results)),
		Nodes: make([]string, 0),
		Links: make([]sankeyLink, 0),
	}
	completeName := func(name string, index int) string {
		return fmt.Sprintf("%s: %s", input.Dimensions[index].String(), name)
	}
	addedNodes := map[string]struct{}{}
	addNode := func(name string) {
		if _, ok := addedNodes[name]; !ok {
			addedNodes[name] = struct{}{}
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
		for i := range len(input.Dimensions) - 1 {
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
