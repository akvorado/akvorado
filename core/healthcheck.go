package core

import (
	"net/http"
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
		// Request a worker to answer
		select {
		case <-c.t.Dying():
			answer(500, "dying")
			return
		case <-time.After(5 * time.Second):
			answer(500, "timeout (no worker)")
			return
		case c.healthy <- answerChan:
		}

		// Wait for answer from worker
		select {
		case <-c.t.Dying():
			answer(500, "dying")
			return
		case <-time.After(5 * time.Second):
			answer(500, "timeout (worker dead)")
			return
		case ok := <-answerChan:
			if ok {
				answer(200, "ok")
			} else {
				answer(500, "nok")
			}
		}
	})
}
