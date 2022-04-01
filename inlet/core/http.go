package core

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// FlowsHTTPHandler streams a JSON copy of all flows just after
// sending them to Kafka. Under load, some flows may not be sent. This
// is intended for debug only.
func (c *Component) FlowsHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var limit, count uint64
		if limitStr := r.FormValue("limit"); limitStr != "" {
			var err error
			limit, err = strconv.ParseUint(limitStr, 10, 64)
			if err != nil {
				limit = 0
			}
		}

		atomic.AddUint32(&c.httpFlowClients, 1)
		defer atomic.AddUint32(&c.httpFlowClients, ^uint32(0))

		w.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(w)
		if limit == 1 {
			encoder.SetIndent("", " ")
		}

		// Flush from time to time
		var tickerChan <-chan time.Time
		wf, ok := w.(http.Flusher)
		if ok {
			tickerChan = time.NewTicker(c.httpFlowFlushDelay).C
		}

		for {
			select {
			case <-c.t.Dying():
				return
			case <-r.Context().Done():
				return
			case msg := <-c.httpFlowChannel:
				encoder.Encode(msg)
				count++
				if limit > 0 && count == limit {
					return
				}
			case <-tickerChan:
				wf.Flush()
			}
		}
	})
}
