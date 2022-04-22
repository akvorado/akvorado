package console

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func (c *Component) widgetFlowLastHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	rows, err := c.d.ClickHouseDB.Conn.Query(ctx,
		`SELECT * FROM flows WHERE TimeReceived = (SELECT MAX(TimeReceived) FROM flows) LIMIT 1`)
	if err != nil {
		c.r.Err(err).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}

	if !rows.Next() {
		gc.JSON(http.StatusNotFound, gin.H{"message": "no flow currently in database."})
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
	row := c.d.ClickHouseDB.Conn.QueryRow(ctx,
		`SELECT COUNT(*)/300 AS rate FROM flows WHERE TimeReceived > date_sub(minute, 5, now())`)
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

	exporters := []struct {
		ExporterName string
	}{}
	err := c.d.ClickHouseDB.Conn.Select(ctx, &exporters,
		`SELECT ExporterName FROM exporters GROUP BY ExporterName ORDER BY ExporterName`)
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
	Name    string `json:"name"`
	Percent uint8  `json:"percent"`
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

	request := fmt.Sprintf(`
WITH
 date_sub(minute, 5, now()) AS StartTime,
 (SELECT SUM(Bytes*SamplingRate) FROM flows WHERE TimeReceived > StartTime %s) AS Total
SELECT
 %s AS Name,
 toUInt8(SUM(Bytes*SamplingRate) / Total * 100) AS Percent
FROM flows
WHERE TimeReceived > StartTime
%s
GROUP BY %s
ORDER BY Percent DESC
LIMIT 5
`, filter, selector, filter, groupby)

	results := []topResult{}
	err := c.d.ClickHouseDB.Conn.Select(ctx, &results, request)
	if err != nil {
		c.r.Err(err).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}
	gc.JSON(http.StatusOK, gin.H{"top": results})
}

func (c *Component) widgetGraphHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())

	width, err := strconv.ParseUint(gc.DefaultQuery("width", "500"), 10, 16)
	if err != nil {
		c.r.Err(err).Msg("invalid width parameter")
		gc.JSON(http.StatusBadRequest, gin.H{"message": "Invalid width value."})
		return
	}
	if width < 5 || width > 1000 {
		gc.JSON(http.StatusBadRequest, gin.H{"message": "Width should be > 5 and < 1000"})
		return
	}
	interval := uint64((24 * time.Hour).Seconds()) / width
	query := fmt.Sprintf(`
SELECT
 toStartOfInterval(TimeReceived, INTERVAL %d second) AS Time,
 SUM(Bytes*SamplingRate*8/%d)/1000/1000/1000 AS Gbps
FROM flows
WHERE TimeReceived > toStartOfInterval(date_sub(hour, 24, now()), INTERVAL %d second)
AND TimeReceived < toStartOfInterval(now(), INTERVAL %d second)
AND InIfBoundary = 'external'
GROUP BY Time
ORDER BY Time`, interval, interval, interval, interval)

	results := []struct {
		Time time.Time `json:"t"`
		Gbps float64   `json:"gbps"`
	}{}
	err = c.d.ClickHouseDB.Conn.Select(ctx, &results, query)
	if err != nil {
		c.r.Err(err).Msg("unable to query database")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to query database."})
		return
	}

	gc.JSON(http.StatusOK, gin.H{"data": results})
}
