package core

import (
	"net/http"
	"sync/atomic"
	"time"

	"akvorado/common/helpers"

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
	var count uint64
	if err := gc.ShouldBindQuery(&params); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
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
			if params.Limit == 1 {
				gc.IndentedJSON(http.StatusOK, msg)
			} else {
				gc.JSON(http.StatusOK, msg)
				gc.Writer.Write([]byte("\n"))
			}
			count++
			if params.Limit > 0 && count == params.Limit {
				return
			}
		case <-tickerChan:
			gc.Writer.Flush()
		}
	}
}
