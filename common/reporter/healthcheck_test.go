// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package reporter_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/reporter"

	"github.com/gin-gonic/gin"
)

func testHealthchecks(ctx context.Context, t *testing.T, r *reporter.Reporter, expected reporter.MultipleHealthcheckResults) {
	t.Helper()
	got := r.RunHealthchecks(ctx)
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("RunHealthchecks() (-got, +want):\n%s", diff)
	}
}

func TestEmptyHealthcheck(t *testing.T) {
	r := reporter.NewMock(t)
	testHealthchecks(context.Background(), t, r,
		reporter.MultipleHealthcheckResults{
			Status:  reporter.HealthcheckOK,
			Details: map[string]reporter.HealthcheckResult{},
		})
}

func TestOneHealthcheck(t *testing.T) {
	r := reporter.NewMock(t)
	r.RegisterHealthcheck("hc1", func(ctx context.Context) reporter.HealthcheckResult {
		return reporter.HealthcheckResult{reporter.HealthcheckOK, "all well"}
	})
	testHealthchecks(context.Background(), t, r,
		reporter.MultipleHealthcheckResults{
			Status: reporter.HealthcheckOK,
			Details: map[string]reporter.HealthcheckResult{
				"hc1": {reporter.HealthcheckOK, "all well"},
			},
		})
}

func TestFailingHealthcheck(t *testing.T) {
	r := reporter.NewMock(t)
	r.RegisterHealthcheck("hc1", func(ctx context.Context) reporter.HealthcheckResult {
		return reporter.HealthcheckResult{reporter.HealthcheckOK, "all well"}
	})
	r.RegisterHealthcheck("hc2", func(ctx context.Context) reporter.HealthcheckResult {
		return reporter.HealthcheckResult{reporter.HealthcheckError, "not so good"}
	})
	testHealthchecks(context.Background(), t, r,
		reporter.MultipleHealthcheckResults{
			Status: reporter.HealthcheckError,
			Details: map[string]reporter.HealthcheckResult{
				"hc1": {reporter.HealthcheckOK, "all well"},
				"hc2": {reporter.HealthcheckError, "not so good"},
			},
		})
}

func TestHealthcheckCancelContext(t *testing.T) {
	r := reporter.NewMock(t)
	r.RegisterHealthcheck("hc1", func(ctx context.Context) reporter.HealthcheckResult {
		return reporter.HealthcheckResult{reporter.HealthcheckOK, "all well"}
	})
	r.RegisterHealthcheck("hc2", func(ctx context.Context) reporter.HealthcheckResult {
		<-ctx.Done()
		return reporter.HealthcheckResult{reporter.HealthcheckError, "I am late, sorry"}
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	testHealthchecks(ctx, t, r,
		reporter.MultipleHealthcheckResults{
			Status: reporter.HealthcheckError,
			Details: map[string]reporter.HealthcheckResult{
				"hc1": {reporter.HealthcheckOK, "all well"},
				"hc2": {reporter.HealthcheckError, "timeout during check"},
			},
		})
}

func TestChannelHealthcheck(t *testing.T) {
	contact := make(chan reporter.ChannelHealthcheckFunc)
	go func() {
		select {
		case f := <-contact:
			f(reporter.HealthcheckOK, "all well, thank you!")
		case <-time.After(50 * time.Millisecond):
		}
	}()

	r := reporter.NewMock(t)
	r.RegisterHealthcheck("hc1", reporter.ChannelHealthcheck(context.Background(), contact))
	testHealthchecks(context.Background(), t, r,
		reporter.MultipleHealthcheckResults{
			Status: reporter.HealthcheckOK,
			Details: map[string]reporter.HealthcheckResult{
				"hc1": {reporter.HealthcheckOK, "all well, thank you!"},
			},
		})
}

func TestHealthcheckHTTPHandler(t *testing.T) {
	r := reporter.NewMock(t)
	r.RegisterHealthcheck("hc1", func(ctx context.Context) reporter.HealthcheckResult {
		return reporter.HealthcheckResult{reporter.HealthcheckOK, "all well"}
	})
	r.RegisterHealthcheck("hc2", func(ctx context.Context) reporter.HealthcheckResult {
		return reporter.HealthcheckResult{reporter.HealthcheckError, "trying to be better"}
	})

	req := httptest.NewRequest("GET", "/api/v0/healthcheck", nil)
	w := httptest.NewRecorder()
	ginRouter := gin.Default()
	ginRouter.GET("/api/v0/healthcheck", r.HealthcheckHTTPHandler)
	ginRouter.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /api/v0/healthcheck status code, got %d, expected %d",
			w.Code, http.StatusServiceUnavailable)
	}

	reader := bufio.NewReader(w.Body)
	decoder := json.NewDecoder(reader)
	var got gin.H
	if err := decoder.Decode(&got); err != nil {
		t.Fatalf("GET /api/v0/healthcheck error:\n%+v", err)
	}
	expected := gin.H{
		"status": "error",
		"details": map[string]any{
			"hc1": map[string]any{
				"status": "ok",
				"reason": "all well",
			},
			"hc2": map[string]any{
				"status": "error",
				"reason": "trying to be better",
			},
		},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("GET /api/v0/healthcheck (-got, +want):\n%s", diff)
	}
}
