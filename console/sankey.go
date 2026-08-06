// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	sb "akvorado/common/sqlbuilder"
	"akvorado/console/query"
)

// graphSankeyHandlerInput describes the input for the /graph/sankey endpoint.
type graphSankeyHandlerInput struct {
	graphCommonHandlerInput
	Bidirectional bool `json:"bidirectional"`
}

// graphSankeyHandlerOutput describes the output for the /graph/sankey endpoint.
type graphSankeyHandlerOutput struct {
	// Unprocessed data for table view
	Rows      [][]string     `json:"rows"`
	Xps       []int          `json:"xps"`  // row → xps
	Axis      []int          `json:"axis"` // row → axis
	AxisNames map[int]string `json:"axis-names"`
	// Processed data for sankey graph
	Nodes []sankeyNode `json:"nodes"`
	Links []sankeyLink `json:"links"`
}
type sankeyNode struct {
	Name string `json:"name"`
	Axis int    `json:"axis"`
}
type sankeyLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Xps    int    `json:"xps"`
	Axis   int    `json:"axis"`
}

// reverseDirection reverts the direction of a provided input. It does not
// modify the original.
func (input graphSankeyHandlerInput) reverseDirection() graphSankeyHandlerInput {
	input.Filter.Swap()
	input.Dimensions = slices.Clone(input.Dimensions)
	query.Columns(input.Dimensions).Reverse(input.schema)
	return input
}

type sankeyToSQL1Options struct {
	skipWithClause   bool
	reverseDirection bool
	// rowsColumns names, for each dimension in input.Dimensions, the column
	// from the forward `rows` CTE that corresponds positionally to it. For the
	// forward query it is left nil and falls back to input.Dimensions. For the
	// reverse query it is the original (forward) dimensions, so the reverse
	// query probes the same `rows` set positionally.
	rowsColumns []query.Column
}

func (input graphSankeyHandlerInput) toSQL1(axis int, res resolution, options sankeyToSQL1Options) *sb.Query {
	r := res.forRange(input.Start, input.End)
	where := r.where(input.Filter)
	rowsColumns := options.rowsColumns
	if rowsColumns == nil {
		rowsColumns = input.Dimensions
	}

	// Units
	units := input.Units
	if options.reverseDirection {
		units = reverseUnits(units)
	}
	unitsSQL := unitsExpr(units)

	// Select
	arrayFields := []sb.Expr{}
	dimensions := []sb.Expr{}
	for i, column := range input.Dimensions {
		arrayFields = append(arrayFields, sb.Function("if",
			sb.Op(sb.Column(column.String()), "IN",
				sb.Select(sb.Column(rowsColumns[i].String())).
					From(sb.Table("rows")).Subquery()),
			column.ToSQLSelect(input.schema, input.database),
			sb.String("Other")))
		dimensions = append(dimensions, sb.Column(column.String()))
	}

	inner := sb.Select(
		sb.Alias(sb.Op(unitsSQL, "/", sb.Column("range")), "xps"),
		sb.Alias(sb.Array(arrayFields...), "dimensions"),
	).
		From(sb.Table("source")).
		Where(where).
		GroupBy(sb.Column("dimensions")).
		OrderBy(sb.Order(sb.Column("xps")).Desc())

	query := sb.Select(
		sb.Alias(sb.Int(int64(axis)), "axis"),
		sb.Star()).
		FromSelect(inner)
	if !options.skipWithClause {
		query.With("source", input.sourceSelect(r.Table))
		// There is no time axis here, so the traffic is averaged over the whole
		// period covered by the data.
		query.WithScalar(
			sb.Select(sb.Op(
				sb.Function("MAX", sb.Column("TimeReceived")), "-",
				sb.Function("MIN", sb.Column("TimeReceived")))).
				From(sb.Table("source")).
				Where(where),
			"range")
		query.With("rows", selectSankeyRowsByLimitType(input, r, dimensions, where, unitsSQL))
	}
	return query
}

// resolveContext returns what is needed to select the table for this query.
// There is no time axis here, so the number of points only steers the table
// selection towards a reasonably consolidated one.
func (input graphSankeyHandlerInput) resolveContext() inputContext {
	return inputContext{
		Start:             input.Start,
		End:               input.End,
		MainTableRequired: requireMainTable(input.schema, input.Dimensions, input.Filter),
		Points:            20,
	}
}

// toSQL converts a sankey query to a list of SQL requests, one per axis.
func (input graphSankeyHandlerInput) toSQL(res resolution) []*sb.Query {
	queries := []*sb.Query{input.toSQL1(1, res, sankeyToSQL1Options{})}
	if input.Bidirectional {
		queries = append(queries, input.reverseDirection().toSQL1(2, res, sankeyToSQL1Options{
			skipWithClause:   true,
			reverseDirection: true,
			rowsColumns:      input.Dimensions,
		}))
	}
	return queries
}

func (c *Component) graphSankeyHandlerFunc(w http.ResponseWriter, req *http.Request) {
	ctx := c.t.Context(req.Context())
	input := graphSankeyHandlerInput{graphCommonHandlerInput: graphCommonHandlerInput{
		schema:   c.d.Schema,
		database: c.d.ClickHouseDB.DatabaseName(),
	}}
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
	if len(input.Dimensions) == 0 {
		// A sankey diagram links dimension values together, there is nothing to
		// draw without a dimension.
		httpserver.WriteJSON(w, http.StatusBadRequest,
			helpers.M{"message": "At least one dimension is required."})
		return
	}

	// Prepare and execute query
	r := c.resolve(input.resolveContext())
	sqlQuery := unionAll(input.toSQL(r))
	w.Header().Set("X-SQL-Query", strings.ReplaceAll(sqlQuery, "\n", "  "))
	results := []struct {
		Axis       uint8    `ch:"axis"`
		Xps        float64  `ch:"xps"`
		Dimensions []string `ch:"dimensions"`
	}{}
	c.metrics.clickhouseQueries.WithLabelValues(r.Table).Inc()
	if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
		c.r.Err(err).Str("query", sqlQuery).Msg("unable to query database")
		httpserver.WriteJSON(w, http.StatusInternalServerError, helpers.M{"message": "Unable to query database."})
		return
	}

	// Prepare output
	output := graphSankeyHandlerOutput{
		Rows:      make([][]string, 0, len(results)),
		Xps:       make([]int, 0, len(results)),
		Axis:      make([]int, 0, len(results)),
		AxisNames: make(map[int]string),
		Nodes:     make([]sankeyNode, 0),
		Links:     make([]sankeyLink, 0),
	}

	// Compute per-axis dimension labels used as node prefixes.
	dimensionLabels := map[int][]string{1: make([]string, len(input.Dimensions))}
	if input.Bidirectional {
		dimensionLabels[2] = make([]string, len(input.Dimensions))
	}
	for i, col := range input.Dimensions {
		dimensionLabels[1][i] = col.String()
		if input.Bidirectional {
			dimensionLabels[2][i] = input.schema.ReverseColumnDirection(col.Key()).String()
		}
	}

	type nodeKey struct {
		name string
		axis int
	}
	addedNodes := map[nodeKey]struct{}{}
	addNode := func(name string, axis int) {
		key := nodeKey{name, axis}
		if _, ok := addedNodes[key]; !ok {
			addedNodes[key] = struct{}{}
			output.Nodes = append(output.Nodes, sankeyNode{Name: name, Axis: axis})
		}
	}
	addLink := func(source, target string, xps, axis int) {
		for idx, link := range output.Links {
			if link.Axis == axis && link.Source == source && link.Target == target {
				output.Links[idx].Xps += xps
				return
			}
		}
		output.Links = append(output.Links, sankeyLink{Source: source, Target: target, Xps: xps, Axis: axis})
	}
	for _, result := range results {
		axis := int(result.Axis)
		output.Rows = append(output.Rows, result.Dimensions)
		output.Xps = append(output.Xps, int(result.Xps))
		output.Axis = append(output.Axis, axis)
		labels := dimensionLabels[axis]
		for i := range len(result.Dimensions) - 1 {
			dimension1 := fmt.Sprintf("%s: %s", labels[i], result.Dimensions[i])
			dimension2 := fmt.Sprintf("%s: %s", labels[i+1], result.Dimensions[i+1])
			addNode(dimension1, axis)
			addNode(dimension2, axis)
			addLink(dimension1, dimension2, int(result.Xps), axis)
		}
	}
	sort.Slice(output.Links, func(i, j int) bool {
		if output.Links[i].Axis != output.Links[j].Axis {
			return output.Links[i].Axis < output.Links[j].Axis
		}
		if output.Links[i].Xps == output.Links[j].Xps {
			return output.Links[i].Source < output.Links[j].Source
		}
		return output.Links[i].Xps > output.Links[j].Xps
	})

	for _, axis := range output.Axis {
		switch axis {
		case 1:
			output.AxisNames[axis] = "Direct"
		case 2:
			output.AxisNames[axis] = "Reverse"
		}
	}

	httpserver.WriteJSON(w, http.StatusOK, output)
}
