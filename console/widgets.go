// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"akvorado/common/schema"
)

func (c *Component) widgetFlowLastHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	replace := []struct {
		key         schema.ColumnKey
		replaceWith string
	}{
		{schema.ColumnDstCommunities, `arrayMap(c -> concat(toString(bitShiftRight(c, 16)), ':',
                      toString(bitAnd(c, 0xffff))), DstCommunities)`},
		{schema.ColumnDstLargeCommunities, `arrayMap(c -> concat(toString(bitAnd(bitShiftRight(c, 64), 0xffffffff)), ':',
                      toString(bitAnd(bitShiftRight(c, 32), 0xffffffff)), ':',
                      toString(bitAnd(c, 0xffffffff))), DstLargeCommunities)`},
		{schema.ColumnSrcMAC, `MACNumToString(SrcMAC)`},
		{schema.ColumnDstMAC, `MACNumToString(DstMAC)`},
	}
	selectClause := []string{"SELECT *"}
	except := []string{}
	for _, r := range replace {
		if column, ok := c.d.Schema.LookupColumnByKey(r.key); ok && !column.Disabled {
			except = append(except, r.key.String())
			selectClause = append(selectClause, fmt.Sprintf("%s AS %s", r.replaceWith, r.key))
		}
	}
	if len(except) > 0 {
		selectClause[0] = fmt.Sprintf("SELECT * EXCEPT (%s)", strings.Join(except, ", "))
	}
	query := fmt.Sprintf(`
%s
FROM flows
WHERE TimeReceived=(SELECT MAX(TimeReceived) FROM flows)
LIMIT 1`, strings.Join(selectClause, ",\n "))
	gc.Header("X-SQL-Query", query)
	// Do not increase counter for this one.
	rows, err := c.d.ClickHouseDB.Conn.Query(ctx, query)
	if err != nil {
		c.r.Err(err).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}

	if !rows.Next() {
		gc.JSON(http.StatusNotFound, gin.H{"message": "No flow currently in database."})
		return
	}
	defer rows.Close()

	var (
		response    = gin.H{}
		columnTypes = rows.ColumnTypes()
		vars        = make([]interface{}, len(columnTypes))
	)
	for i := range columnTypes {
		vars[i] = reflect.New(columnTypes[i].ScanType()).Interface()
	}
	if err := rows.Scan(vars...); err != nil {
		c.r.Err(err).Msg("unable to parse flow")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to parse flow."})
		return
	}
	for index, column := range rows.Columns() {
		response[column] = vars[index]
	}
	gc.IndentedJSON(http.StatusOK, response)
}

func (c *Component) widgetFlowRateHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	query := `SELECT COUNT(*)/300 AS rate FROM flows WHERE TimeReceived > date_sub(minute, 5, now())`
	gc.Header("X-SQL-Query", query)
	// Do not increase counter for this one.
	row := c.d.ClickHouseDB.Conn.QueryRow(ctx, query)
	if err := row.Err(); err != nil {
		c.r.Err(err).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}
	var result float64
	if err := row.Scan(&result); err != nil {
		c.r.Err(err).Msg("unable to parse result")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to parse result."})
		return
	}
	gc.IndentedJSON(http.StatusOK, gin.H{
		"rate":   result,
		"period": "second",
	})
}

func (c *Component) widgetExportersHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	query := `SELECT ExporterName FROM exporters GROUP BY ExporterName ORDER BY ExporterName`
	gc.Header("X-SQL-Query", query)
	// Do not increase counter for this one.

	exporters := []struct {
		ExporterName string
	}{}
	err := c.d.ClickHouseDB.Conn.Select(ctx, &exporters, query)
	if err != nil {
		c.r.Err(err).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}
	exporterList := make([]string, len(exporters))
	for idx, exporter := range exporters {
		exporterList[idx] = exporter.ExporterName
	}

	gc.IndentedJSON(http.StatusOK, gin.H{"exporters": exporterList})
}

type topResult struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

func (c *Component) widgetTopHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	var (
		selector          string
		groupby           string
		filter            string
		mainTableRequired bool
	)

	switch gc.Param("name") {
	default:
		gc.JSON(http.StatusNotFound, gin.H{"message": "Unknown top request."})
		return
	case "src-as":
		selector = `concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???'))`
		groupby = `SrcAS`
		filter = "AND InIfBoundary = 'external'"
	case "dst-as":
		selector = `concat(toString(DstAS), ': ', dictGetOrDefault('asns', 'name', DstAS, '???'))`
		groupby = `DstAS`
		filter = "AND OutIfBoundary = 'external'"
	case "src-country":
		selector = `SrcCountry`
		filter = "AND InIfBoundary = 'external'"
	case "dst-country":
		selector = `DstCountry`
		filter = "AND OutIfBoundary = 'external'"
	case "exporter":
		selector = "ExporterName"
	case "protocol":
		selector = `dictGetOrDefault('protocols', 'name', Proto, '???')`
		groupby = `Proto`
	case "etype":
		selector = `if(equals(EType, 34525), 'IPv6', if(equals(EType, 2048), 'IPv4', '???'))`
		groupby = `EType`
	case "src-port":
		selector = `concat(dictGetOrDefault('protocols', 'name', Proto, '???'), '/', toString(SrcPort))`
		groupby = `Proto, SrcPort`
		mainTableRequired = true
	case "dst-port":
		selector = `concat(dictGetOrDefault('protocols', 'name', Proto, '???'), '/', toString(DstPort))`
		groupby = `Proto, DstPort`
		mainTableRequired = true
	}
	if groupby == "" {
		groupby = selector
	}

	now := c.d.Clock.Now()
	query := c.finalizeQuery(fmt.Sprintf(`
{{ with %s }}
WITH
 (SELECT SUM(Bytes*SamplingRate) FROM {{ .Table }} WHERE {{ .Timefilter }} %s) AS Total
SELECT
 if(empty(%s),'Unknown',%s) AS Name,
 SUM(Bytes*SamplingRate) / Total * 100 AS Percent
FROM {{ .Table }}
WHERE {{ .Timefilter }}
%s
GROUP BY %s
ORDER BY Percent DESC
LIMIT 5
{{ end }}`,
		templateContext(inputContext{
			Start:             now.Add(-5 * time.Minute),
			End:               now,
			MainTableRequired: mainTableRequired,
			Points:            5,
		}),
		filter, selector, selector, filter, groupby))
	gc.Header("X-SQL-Query", query)

	results := []topResult{}
	err := c.d.ClickHouseDB.Conn.Select(ctx, &results, strings.TrimSpace(query))
	if err != nil {
		c.r.Err(err).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}
	gc.JSON(http.StatusOK, gin.H{"top": results})
}

func (c *Component) widgetGraphHandlerFunc(gc *gin.Context) {
	// first step: define which filter to use
	filter := c.config.HomepageGraphFilter
	if filter != "" {
		filter = fmt.Sprintf("AND %s", filter)
	}
	ctx := c.t.Context(gc.Request.Context())
	now := c.d.Clock.Now()
	query := c.finalizeQuery(fmt.Sprintf(`
{{ with %s }}
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS Time,
 SUM(Bytes*SamplingRate*8/{{ .Interval }})/1000/1000/1000 AS Gbps
FROM {{ .Table }}
WHERE {{ .Timefilter }}
%s
GROUP BY Time
ORDER BY Time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
{{ end }}`,
		templateContext(inputContext{
			Start:             now.Add(-24 * time.Hour),
			End:               now,
			MainTableRequired: false,
			Points:            200,
		}),
		filter))
	gc.Header("X-SQL-Query", query)

	results := []struct {
		Time time.Time `json:"t"`
		Gbps float64   `json:"gbps"`
	}{}
	err := c.d.ClickHouseDB.Conn.Select(ctx, &results, strings.TrimSpace(query))
	if err != nil {
		c.r.Err(err).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}

	gc.JSON(http.StatusOK, gin.H{"data": results})
}
