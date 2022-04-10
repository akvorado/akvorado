package core

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type flowsParameters struct {
	Limit uint64 `form:"limit"`
}

// FlowsHTTPHandler streams a JSON copy of all flows just after
// sending them to Kafka. Under load, some flows may not be sent. This
// is intended for debug only.
func (c *Component) FlowsHTTPHandler(gc *gin.Context) {
	var params flowsParameters
	var limit, count uint64
	if gc.ShouldBindQuery(&params) == nil {
		limit = params.Limit
	}

	atomic.AddUint32(&c.httpFlowClients, 1)
	defer atomic.AddUint32(&c.httpFlowClients, ^uint32(0))

	// Flush from time to time
	var tickerChan <-chan time.Time
	tickerChan = time.NewTicker(c.httpFlowFlushDelay).C

	for {
		select {
		case <-c.t.Dying():
			return
		case <-gc.Request.Context().Done():
			return
		case msg := <-c.httpFlowChannel:
			if limit == 1 {
				gc.IndentedJSON(http.StatusOK, msg)
			} else {
				gc.JSON(http.StatusOK, msg)
				gc.Writer.Write([]byte("\n"))
			}
			count++
			if limit > 0 && count == limit {
				return
			}
		case <-tickerChan:
			gc.Writer.Flush()
		}
	}
}
