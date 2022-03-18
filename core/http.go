package core

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// HealthcheckHTTPHandler returns an handler for healthchecks. It
// checks if at least one worker is alive.
func (c *Component) HealthcheckHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		answerChan := make(chan bool)

		answer := func(code int, text string) {
			w.WriteHeader(code)
			w.Write([]byte(text))
		}
		if !c.t.Alive() {
			answer(http.StatusServiceUnavailable, "dead")
			return
		}

		// Request a worker to answer
		select {
		case <-c.t.Dying():
			answer(http.StatusServiceUnavailable, "dying")
			return
		case <-time.After(5 * time.Second):
			answer(http.StatusServiceUnavailable, "timeout (no worker)")
			return
		case c.healthy <- answerChan:
		}

		// Wait for answer from worker
		select {
		case <-c.t.Dying():
			answer(http.StatusServiceUnavailable, "dying")
			return
		case <-time.After(5 * time.Second):
			answer(http.StatusServiceUnavailable, "timeout (worker dead)")
			return
		case ok := <-answerChan:
			if !ok {
				answer(http.StatusInternalServerError, "nok")
				return
			}
			answer(http.StatusOK, "ok")
		}
	})
}

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
		w.WriteHeader(http.StatusOK)
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
