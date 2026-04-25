// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/httpserver"
)

// FlowsHTTPHandler streams a JSON copy of all flows just after
// sending them to ClickHouse. Under load, some flows may not be sent. This
// is intended for debug only.
func (c *Component) FlowsHTTPHandler(w http.ResponseWriter, req *http.Request) {
	var limit uint64
	if raw := req.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			httpserver.WriteJSON(w, http.StatusBadRequest, helpers.M{
				"message": "Invalid limit",
			})
			return
		}
		limit = parsed
	}

	var count uint64
	dying := c.t.Dying()

	atomic.AddUint32(&c.httpFlowClients, 1)
	defer atomic.AddUint32(&c.httpFlowClients, ^uint32(0))

	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "application/json")

	// Flush from time to time
	var tickerChan <-chan time.Time
	ticker := time.NewTicker(c.httpFlowFlushDelay)
	tickerChan = ticker.C
	defer ticker.Stop()

	for {
		select {
		case <-dying:
			return
		case <-req.Context().Done():
			return
		case msg := <-c.httpFlowChannel:
			w.Write(msg)
			w.Write([]byte("\n"))

			count++
			if limit > 0 && count == limit {
				return
			}
		case <-tickerChan:
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}
