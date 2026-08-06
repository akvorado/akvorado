// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"net/http"
	"reflect"
	"strings"
	"time"

	"akvorado/common/constants"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/schema"
	sb "akvorado/common/sqlbuilder"
	"akvorado/console/query"
)

func (c *Component) widgetFlowLastHandlerFunc(w http.ResponseWriter, req *http.Request) {
	ctx := c.t.Context(req.Context())
	replace := []struct {
		key         schema.ColumnKey
		replaceWith sb.Expr
	}{
		{schema.ColumnSrcCommunities, query.CommunitiesToStrings("SrcCommunities")},
		{schema.ColumnSrcLargeCommunities, query.LargeCommunitiesToStrings("SrcLargeCommunities")},
		{schema.ColumnDstCommunities, query.CommunitiesToStrings("DstCommunities")},
		{schema.ColumnDstLargeCommunities, query.LargeCommunitiesToStrings("DstLargeCommunities")},
		{schema.ColumnSrcMAC, sb.Function("MACNumToString", sb.Column("SrcMAC"))},
		{schema.ColumnDstMAC, sb.Function("MACNumToString", sb.Column("DstMAC"))},
	}
	// The columns below are not readable as they are stored, so they are
	// dropped from the "*" and added back in a friendlier form.
	replaced := []sb.Expr{}
	except := []string{}
	for _, r := range replace {
		if column, ok := c.d.Schema.LookupColumnByKey(r.key); ok && !column.Disabled {
			except = append(except, r.key.String())
			replaced = append(replaced, sb.Alias(r.replaceWith, r.key.String()))
		}
	}
	last := sb.Select()
	if len(except) > 0 {
		last.Item(sb.Star(), sb.Except(except...))
	} else {
		last.Item(sb.Star())
	}
	for _, expr := range replaced {
		last.Item(expr)
	}
	sqlQuery := last.
		From(sb.Table("flows")).
		Where(sb.Op(sb.Column("TimeReceived"), "=",
			sb.Select(sb.Function("MAX", sb.Column("TimeReceived"))).
				From(sb.Table("flows")).Subquery())).
		Limit(1).
		String()
	w.Header().Set("X-SQL-Query", sqlQuery)
	// Do not increase counter for this one.
	rows, err := c.d.ClickHouseDB.Conn.Query(ctx, sqlQuery)
	if err != nil {
		c.r.Err(err).Msg("unable to query database")
		httpserver.WriteJSON(w, http.StatusInternalServerError, helpers.M{"message": "Unable to query database."})
		return
	}
	defer rows.Close()

	if !rows.Next() {
		httpserver.WriteJSON(w, http.StatusNotFound, helpers.M{"message": "No flow currently in database."})
		return
	}

	var (
		response    = helpers.M{}
		columnTypes = rows.ColumnTypes()
		vars        = make([]any, len(columnTypes))
	)
	for i := range columnTypes {
		vars[i] = reflect.New(columnTypes[i].ScanType()).Interface()
	}
	if err := rows.Scan(vars...); err != nil {
		c.r.Err(err).Msg("unable to parse flow")
		httpserver.WriteJSON(w, http.StatusInternalServerError, helpers.M{"message": "Unable to parse flow."})
		return
	}
	for index, column := range rows.Columns() {
		response[column] = vars[index]
	}
	httpserver.WriteIndentedJSON(w, http.StatusOK, response)
}

func (c *Component) widgetFlowRateHandlerFunc(w http.ResponseWriter, req *http.Request) {
	ctx := c.t.Context(req.Context())
	query := `SELECT COUNT(*)/300 AS rate FROM flows WHERE TimeReceived > date_sub(minute, 5, now())`
	w.Header().Set("X-SQL-Query", query)
	// Do not increase counter for this one.
	var result float64
	row := c.d.ClickHouseDB.Conn.QueryRow(ctx, query)
	if err := row.Scan(&result); err != nil {
		c.r.Err(err).Msg("unable to parse result")
		httpserver.WriteJSON(w, http.StatusInternalServerError, helpers.M{"message": "Unable to parse result."})
		return
	}
	httpserver.WriteIndentedJSON(w, http.StatusOK, helpers.M{
		"rate":   result,
		"period": "second",
	})
}

func (c *Component) widgetExportersHandlerFunc(w http.ResponseWriter, req *http.Request) {
	ctx := c.t.Context(req.Context())
	query := `SELECT ExporterName FROM exporters GROUP BY ExporterName ORDER BY ExporterName`
	w.Header().Set("X-SQL-Query", query)
	// Do not increase counter for this one.

	exporters := []struct {
		ExporterName string
	}{}
	err := c.d.ClickHouseDB.Conn.Select(ctx, &exporters, query)
	if err != nil {
		c.r.Err(err).Msg("unable to query database")
		httpserver.WriteJSON(w, http.StatusInternalServerError, helpers.M{"message": "Unable to query database."})
		return
	}
	exporterList := make([]string, len(exporters))
	for idx, exporter := range exporters {
		exporterList[idx] = exporter.ExporterName
	}

	httpserver.WriteIndentedJSON(w, http.StatusOK, helpers.M{"exporters": exporterList})
}

type topResult struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

func (c *Component) widgetTopHandlerFunc(w http.ResponseWriter, req *http.Request) {
	ctx := c.t.Context(req.Context())
	var (
		selector          sb.Expr
		groupby           []sb.Expr
		filter            sb.Expr
		mainTableRequired bool
	)
	dictLookup := func(dictionary string, column string) sb.Expr {
		return sb.Column(column).Apply(query.DictionaryLookup(c.d.Schema, dictionary, "???"))
	}

	rawName := req.PathValue("name")
	widgetName, err := HomepageTopWidgetString(rawName)
	if err != nil {
		httpserver.WriteJSON(w, http.StatusBadRequest, helpers.M{"message": helpers.Capitalize(err.Error())})
		return
	}

	switch widgetName {
	case HomepageTopWidgetSrcAS, HomepageTopWidgetDstAS:
		column := "SrcAS"
		if widgetName == HomepageTopWidgetDstAS {
			column = "DstAS"
		}
		selector = sb.Function("concat",
			sb.Function("toString", sb.Column(column)),
			sb.String(": "),
			dictLookup(schema.DictionaryASNs, column))
		groupby = sb.Columns(column)
	case HomepageTopWidgetSrcCountry:
		selector = sb.Column("SrcCountry")
	case HomepageTopWidgetDstCountry:
		selector = sb.Column("DstCountry")
	case HomepageTopWidgetExporter:
		selector = sb.Column("ExporterName")
	case HomepageTopWidgetProtocol:
		selector = dictLookup(schema.DictionaryProtocols, "Proto")
		groupby = sb.Columns("Proto")
	case HomepageTopWidgetEtype:
		etype := sb.Column("EType")
		selector = sb.Function("if",
			sb.Function("equals", etype, sb.Uint(constants.ETypeIPv6)),
			sb.String("IPv6"),
			sb.Function("if",
				sb.Function("equals", etype, sb.Uint(constants.ETypeIPv4)),
				sb.String("IPv4"),
				sb.String("???")))
		groupby = sb.Columns("EType")
	case HomepageTopWidgetSrcPort, HomepageTopWidgetDstPort:
		column := "SrcPort"
		if widgetName == HomepageTopWidgetDstPort {
			column = "DstPort"
		}
		selector = sb.Function("concat",
			dictLookup(schema.DictionaryProtocols, "Proto"),
			sb.String("/"),
			sb.Function("toString", sb.Column(column)))
		groupby = sb.Columns("Proto", column)
		mainTableRequired = true
	default:
		httpserver.WriteJSON(w, http.StatusNotFound, helpers.M{"message": "Unknown top request."})
		return
	}
	if strings.HasPrefix(rawName, "src-") {
		filter = sb.Op(sb.Column("InIfBoundary"), "=", sb.String("external"))
	} else if strings.HasPrefix(rawName, "dst-") {
		filter = sb.Op(sb.Column("OutIfBoundary"), "=", sb.String("external"))
	}
	if len(groupby) == 0 {
		groupby = []sb.Expr{selector}
	}

	now := c.d.Clock.Now()
	start, end := now.Add(-5*time.Minute), now
	r := c.resolve(inputContext{
		Start:             start,
		End:               end,
		MainTableRequired: mainTableRequired,
		Points:            5,
	}).forRange(start, end)
	where := sb.And(r.timefilter(), filter)
	bytes := sb.MustParseExpr("SUM(Bytes*SamplingRate)")
	sqlQuery := sb.Select(
		sb.Alias(sb.Function("if",
			sb.Function("empty", selector),
			sb.String("Unknown"),
			selector), "Name"),
		sb.Alias(sb.Op(
			sb.Op(bytes, "/", sb.Column("Total")), "*", sb.Uint(100)),
			"Percent")).
		WithScalar(sb.Select(bytes).From(sb.Table(r.Table)).Where(where), "Total").
		From(sb.Table(r.Table)).
		Where(where).
		GroupBy(groupby...).
		OrderBy(sb.Order(sb.Column("Percent")).Desc()).
		Limit(5).
		String()
	w.Header().Set("X-SQL-Query", sqlQuery)

	results := []topResult{}
	c.metrics.clickhouseQueries.WithLabelValues(r.Table).Inc()
	if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
		c.r.Err(err).Msg("unable to query database")
		httpserver.WriteJSON(w, http.StatusInternalServerError, helpers.M{"message": "Unable to query database."})
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, helpers.M{"top": results})
}

func (c *Component) widgetGraphHandlerFunc(w http.ResponseWriter, req *http.Request) {
	ctx := c.t.Context(req.Context())
	now := c.d.Clock.Now()
	start, end := now.Add(-c.config.HomepageGraphTimeRange), now
	r := c.resolve(inputContext{
		Start:             start,
		End:               end,
		MainTableRequired: false,
		Points:            200,
	}).forRange(start, end)
	gbps := sb.Function("SUM",
		sb.Op(sb.MustParseExpr("Bytes*SamplingRate*8"), "/", sb.Uint(r.Interval)))
	// From bits per second to gigabits per second.
	for range 3 {
		gbps = sb.Op(gbps, "/", sb.Uint(1000))
	}
	sqlQuery := sb.Select(
		sb.Alias(r.toStartOfInterval(), "Time"),
		sb.Alias(gbps, "Gbps")).
		From(sb.Table(r.Table)).
		Where(sb.And(r.timefilter(), c.homepageGraphFilter)).
		GroupBy(sb.Column("Time")).
		OrderBy(sb.Order(sb.Column("Time")).Fill(
			r.timefilterStart(),
			sb.Op(r.timefilterEnd(), "+", seconds(1)),
			sb.Uint(r.Interval))).
		String()
	w.Header().Set("X-SQL-Query", sqlQuery)

	results := []struct {
		Time time.Time `json:"t"`
		Gbps float64   `json:"gbps"`
	}{}
	c.metrics.clickhouseQueries.WithLabelValues(r.Table).Inc()
	err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery)
	if err != nil {
		c.r.Err(err).Msg("unable to query database")
		httpserver.WriteJSON(w, http.StatusInternalServerError, helpers.M{"message": "Unable to query database."})
		return
	}

	httpserver.WriteJSON(w, http.StatusOK, helpers.M{"data": results})
}
