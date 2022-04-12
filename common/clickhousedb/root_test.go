package clickhousedb

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestMock(t *testing.T) {
	r := reporter.NewMock(t)
	chComponent, mock := NewMock(t, r)

	// Check a select query (this is a bit dumb, but it shows how gomock works)
	t.Run("select", func(t *testing.T) {
		var got []struct {
			N uint64 `ch:"n"`
			M uint64 `ch:"m"`
		}
		expected := []struct {
			N uint64 `ch:"n"`
			M uint64 `ch:"m"`
		}{
			{0, 1},
			{1, 2},
			{2, 3},
			{3, 4},
			{4, 5},
		}
		mock.EXPECT().
			Select(gomock.Any(), gomock.Any(), "SELECT number as n, number + 1 as m FROM numbers(5)").
			SetArg(1, expected).
			Return(nil)

		err := chComponent.Select(context.Background(), &got, "SELECT number as n, number + 1 as m FROM numbers(5)")
		if err != nil {
			t.Fatalf("SELECT error:\n%+v", err)
		}

		if diff := helpers.Diff(got, expected); diff != "" {
			t.Fatalf("SELECT (-got, +want):\n%s", diff)
		}
	})

	// Check healthcheck
	t.Run("healthcheck", func(t *testing.T) {
		firstCall := mock.EXPECT().
			Query(gomock.Any(), "SELECT 1").
			Return(nil, nil)
		got := r.RunHealthchecks(context.Background())
		if diff := helpers.Diff(got.Details["clickhousedb"], reporter.HealthcheckResult{
			Status: reporter.HealthcheckOK,
			Reason: "database available",
		}); diff != "" {
			t.Fatalf("runHealthcheck() (-got, +want):\n%s", diff)
		}

		mock.EXPECT().
			Query(gomock.Any(), "SELECT 1").
			Return(nil, errors.New("not available")).
			After(firstCall)
		got = r.RunHealthchecks(context.Background())
		if diff := helpers.Diff(got.Details["clickhousedb"], reporter.HealthcheckResult{
			Status: reporter.HealthcheckWarning,
			Reason: "database unavailable",
		}); diff != "" {
			t.Fatalf("runHealthcheck() (-got, +want):\n%s", diff)
		}
	})
}

func TestRealClickHouse(t *testing.T) {
	r := reporter.NewMock(t)
	chComponent := SetupClickHouse(t, r)

	// Check a select query
	t.Run("select", func(t *testing.T) {
		var got []struct {
			N uint64 `ch:"n"`
			M uint64 `ch:"m"`
		}
		err := chComponent.Select(context.Background(), &got, "SELECT number as n, number + 1 as m FROM numbers(5)")
		if err != nil {
			t.Fatalf("SELECT error:\n%+v", err)
		}

		expected := []struct {
			N uint64
			M uint64
		}{
			{0, 1},
			{1, 2},
			{2, 3},
			{3, 4},
			{4, 5},
		}
		if diff := helpers.Diff(got, expected); diff != "" {
			t.Fatalf("SELECT (-got, +want):\n%s", diff)
		}
	})

	// Check healthcheck
	t.Run("healthcheck", func(t *testing.T) {
		got := r.RunHealthchecks(context.Background())
		if diff := helpers.Diff(got.Details["clickhousedb"], reporter.HealthcheckResult{
			Status: reporter.HealthcheckOK,
			Reason: "database available",
		}); diff != "" {
			t.Fatalf("runHealthcheck() (-got, +want):\n%s", diff)
		}
	})
}
