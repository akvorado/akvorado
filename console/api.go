package console

import (
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
)

func (c *Component) lastFlowHandlerFunc(gc *gin.Context) {
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
