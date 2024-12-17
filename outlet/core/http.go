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
// sending them to ClickHouse. Under load, some flows may not be sent. This
// is intended for debug only.
func (c *Component) FlowsHTTPHandler(gc *gin.Context) {
	var params flowsParameters
	var count uint64
	if err := gc.ShouldBindQuery(&params); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}
	dying := c.t.Dying()

	atomic.AddUint32(&c.httpFlowClients, 1)
	defer atomic.AddUint32(&c.httpFlowClients, ^uint32(0))

	// Flush from time to time
	var tickerChan <-chan time.Time
	ticker := time.NewTicker(c.httpFlowFlushDelay)
	tickerChan = ticker.C
	defer ticker.Stop()

	for {
		select {
		case <-dying:
			return
		case <-gc.Request.Context().Done():
			return
		case msg := <-c.httpFlowChannel:
			gc.Header("Content-Type", "application/json")
			gc.Status(http.StatusOK)
			gc.Writer.Write(msg)
			gc.Writer.Write([]byte("\n"))

			count++
			if params.Limit > 0 && count == params.Limit {
				return
			}
		case <-tickerChan:
			gc.Writer.Flush()
		}
	}
}
