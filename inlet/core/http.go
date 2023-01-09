// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

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
	format := gc.NegotiateFormat("application/json", "application/x-protobuf")

	atomic.AddUint32(&c.httpFlowClients, 1)
	defer atomic.AddUint32(&c.httpFlowClients, ^uint32(0))

	// Flush from time to time
	var tickerChan <-chan time.Time
	ticker := time.NewTicker(c.httpFlowFlushDelay)
	tickerChan = ticker.C
	defer ticker.Stop()

	for {
		select {
		case <-c.t.Dying():
			return
		case <-gc.Request.Context().Done():
			return
		case msg := <-c.httpFlowChannel:
			switch format {
			case "application/json":
				if params.Limit == 1 {
					gc.IndentedJSON(http.StatusOK, msg)
				} else {
					gc.JSON(http.StatusOK, msg)
					gc.Writer.Write([]byte("\n"))
				}
			case "application/x-protobuf":
				buf, err := msg.EncodeMessage()
				if err != nil {
					continue
				}
				gc.Set("Content-Type", format)
				gc.Writer.Write(buf)
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
