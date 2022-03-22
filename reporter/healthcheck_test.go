package reporter_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"akvorado/helpers"
	"akvorado/reporter"
)

func testHealthchecks(t *testing.T, r *reporter.Reporter, ctx context.Context, expectedStatus reporter.HealthcheckStatus, expectedResults map[string]reporter.HealthcheckResult) {
	t.Helper()
	gotStatus, gotResults := r.RunHealthchecks(ctx)
	if gotStatus != expectedStatus {
		t.Errorf("RunHealthchecks() status got %s expected %s", gotStatus, expectedStatus)
	}
	if diff := helpers.Diff(gotResults, expectedResults); diff != "" {
		t.Errorf("RunHealthchecks() (-got, +want):\n%s", diff)
	}
}

func TestEmptyHealthcheck(t *testing.T) {
	r := reporter.NewMock(t)
	testHealthchecks(t, r, context.Background(),
		reporter.HealthcheckOK, map[string]reporter.HealthcheckResult{})
}

func TestOneHealthcheck(t *testing.T) {
	r := reporter.NewMock(t)
	r.RegisterHealthcheck("hc1", func(ctx context.Context) reporter.HealthcheckResult {
		return reporter.HealthcheckResult{reporter.HealthcheckOK, "all well"}
	})
	testHealthchecks(t, r, context.Background(),
		reporter.HealthcheckOK, map[string]reporter.HealthcheckResult{
			"hc1": {reporter.HealthcheckOK, "all well"},
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
	testHealthchecks(t, r, context.Background(),
		reporter.HealthcheckError, map[string]reporter.HealthcheckResult{
			"hc1": {reporter.HealthcheckOK, "all well"},
			"hc2": {reporter.HealthcheckError, "not so good"},
		})
}

func TestHealthcheckCancelContext(t *testing.T) {
	r := reporter.NewMock(t)
	r.RegisterHealthcheck("hc1", func(ctx context.Context) reporter.HealthcheckResult {
		return reporter.HealthcheckResult{reporter.HealthcheckOK, "all well"}
	})
	r.RegisterHealthcheck("hc2", func(ctx context.Context) reporter.HealthcheckResult {
		select {
		case <-ctx.Done():
			return reporter.HealthcheckResult{reporter.HealthcheckError, "I am late, sorry"}
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	testHealthchecks(t, r, ctx,
		reporter.HealthcheckError, map[string]reporter.HealthcheckResult{
			"hc1": {reporter.HealthcheckOK, "all well"},
			"hc2": {reporter.HealthcheckError, "timeout during check"},
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
	testHealthchecks(t, r, context.Background(),
		reporter.HealthcheckOK, map[string]reporter.HealthcheckResult{
			"hc1": {reporter.HealthcheckOK, "all well, thank you!"},
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
	r.HealthcheckHTTPHandler().ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /api/v0/healthcheck status code, got %d, expected %d",
			w.Code, http.StatusServiceUnavailable)
	}

	reader := bufio.NewReader(w.Body)
	decoder := json.NewDecoder(reader)
	var got map[string]interface{}
	if err := decoder.Decode(&got); err != nil {
		t.Fatalf("GET /api/v0/healthcheck error:\n%+v", err)
	}
	expected := map[string]interface{}{
		"status": "error",
		"details": map[string]interface{}{
			"hc1": map[string]string{
				"status": "ok",
				"reason": "all well",
			},
			"hc2": map[string]string{
				"status": "error",
				"reason": "trying to be better",
			},
		},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("GET /api/v0/healthcheck (-got, +want):\n%s", diff)
	}
}
