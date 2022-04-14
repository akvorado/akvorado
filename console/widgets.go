package console

import (
	"net/http"
	"reflect"

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
