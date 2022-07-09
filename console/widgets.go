// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"net/http"
	"reflect"
	"time"

	"akvorado/common/helpers"

	"github.com/gin-gonic/gin"
)

func (c *Component) widgetFlowLastHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	query := `SELECT * FROM flows WHERE TimeReceived = (SELECT MAX(TimeReceived) FROM flows) LIMIT 1`
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
		selector string
		groupby  string
		filter   string
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
	case "dst-port":
		selector = `concat(dictGetOrDefault('protocols', 'name', Proto, '???'), '/', toString(DstPort))`
		groupby = `Proto, DstPort`
	}
	if groupby == "" {
		groupby = selector
	}

	now := c.d.Clock.Now()
	query := c.queryFlowsTable(fmt.Sprintf(`
WITH
 (SELECT SUM(Bytes*SamplingRate) FROM {table} WHERE {timefilter} %s) AS Total
SELECT
 %s AS Name,
 SUM(Bytes*SamplingRate) / Total * 100 AS Percent
FROM {table}
WHERE {timefilter}
%s
GROUP BY %s
ORDER BY Percent DESC
LIMIT 5
`, filter, selector, filter, groupby), now.Add(-5*time.Minute), now, time.Minute)
	gc.Header("X-SQL-Query", query)

	results := []topResult{}
	err := c.d.ClickHouseDB.Conn.Select(ctx, &results, query)
	if err != nil {
		c.r.Err(err).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}
	gc.JSON(http.StatusOK, gin.H{"top": results})
}

type widgetParameters struct {
	Points uint64 `form:"points" binding:"isdefault|min=5,max=1000"`
}

func (c *Component) widgetGraphHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())

	var params widgetParameters
	if err := gc.ShouldBindQuery(&params); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}
	if params.Points == 0 {
		params.Points = 200
	}
	interval := int64((24 * time.Hour).Seconds()) / int64(params.Points)
	slot := fmt.Sprintf(`max2(intDiv(%d, {resolution})*{resolution}, 1)`, interval)
	now := c.d.Clock.Now()
	query := c.queryFlowsTable(fmt.Sprintf(`
SELECT
 toStartOfInterval(TimeReceived, INTERVAL %s second) AS Time,
 SUM(Bytes*SamplingRate*8/%s)/1000/1000/1000 AS Gbps
FROM {table}
WHERE {timefilter}
AND InIfBoundary = 'external'
GROUP BY Time
ORDER BY Time WITH FILL
 FROM toStartOfInterval({timefilter.Start}, INTERVAL %s second)
 TO {timefilter.Stop}
 STEP toUInt32(%s)`, slot, slot, slot, slot), now.Add(-24*time.Hour), now, time.Duration(interval)*time.Second)
	gc.Header("X-SQL-Query", query)

	results := []struct {
		Time time.Time `json:"t"`
		Gbps float64   `json:"gbps"`
	}{}
	err := c.d.ClickHouseDB.Conn.Select(ctx, &results, query)
	if err != nil {
		c.r.Err(err).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}

	gc.JSON(http.StatusOK, gin.H{"data": results})
}
