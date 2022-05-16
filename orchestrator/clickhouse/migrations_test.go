package clickhouse

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestGetHTTPBaseURL(t *testing.T) {
	r := reporter.NewMock(t)
	http := http.NewMock(t, r)
	c, err := New(r, DefaultConfiguration(), Dependencies{
		Daemon: daemon.NewMock(t),
		HTTP:   http,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	rawURL, err := c.getHTTPBaseURL("8.8.8.8:9000")
	if err != nil {
		t.Fatalf("getHTTPBaseURL() error:\n%+v", err)
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("Parse(%q) error:\n%+v", rawURL, err)
	}
	expectedURL := url.URL{
		Scheme: "http",
		Host:   http.Address.String(),
	}
	parsedURL.Host = parsedURL.Host[strings.LastIndex(parsedURL.Host, ":"):]
	expectedURL.Host = expectedURL.Host[strings.LastIndex(expectedURL.Host, ":"):]
	// We can't really know our IP
	if diff := helpers.Diff(parsedURL, expectedURL); diff != "" {
		t.Fatalf("getHTTPBaseURL() (-got, +want):\n%s", diff)
	}
}

func TestMigration(t *testing.T) {
	r := reporter.NewMock(t)
	chComponent := clickhousedb.SetupClickHouse(t, r)

	func() {
		// First time
		configuration := DefaultConfiguration()
		configuration.OrchestratorURL = "http://something"
		ch, err := New(r, configuration, Dependencies{
			Daemon:     daemon.NewMock(t),
			HTTP:       http.NewMock(t, r),
			ClickHouse: chComponent,
		})
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
		helpers.StartStop(t, ch)
		select {
		case <-ch.migrationsDone:
		case <-time.After(3 * time.Second):
			t.Fatalf("Migrations not done")
		}

		// Check with the ClickHouse client we have our tables
		rows, err := chComponent.Query(context.Background(), "SHOW TABLES")
		if err != nil {
			t.Fatalf("Query() error:\n%+v", err)
		}
		got := []string{}
		for rows.Next() {
			var table string
			if err := rows.Scan(&table); err != nil {
				t.Fatalf("Scan() error:\n%+v", err)
			}
			if !strings.HasPrefix(table, ".") {
				got = append(got, table)
			}
		}
		expected := []string{
			"asns",
			"exporters",
			"flows",
			"flows_1_raw",
			"flows_1_raw_consumer",
			"flows_1h0m0s",
			"flows_1h0m0s_consumer",
			"flows_1m0s",
			"flows_1m0s_consumer",
			"flows_5m0s",
			"flows_5m0s_consumer",
			"protocols",
		}
		if diff := helpers.Diff(got, expected); diff != "" {
			t.Fatalf("SHOW TABLES (-got, +want):\n%s", diff)
		}
	}()

	func() {
		// Second time
		r := reporter.NewMock(t)
		configuration := DefaultConfiguration()
		configuration.OrchestratorURL = "http://something"
		ch, err := New(r, configuration, Dependencies{
			Daemon:     daemon.NewMock(t),
			HTTP:       http.NewMock(t, r),
			ClickHouse: chComponent,
		})
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
		helpers.StartStop(t, ch)
		select {
		case <-ch.migrationsDone:
		case <-time.After(3 * time.Second):
			t.Fatalf("Migrations not done")
		}

		// No migration should have been applied the second time
		gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_",
			"applied_steps")
		expectedMetrics := map[string]string{
			`applied_steps`: "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

	}()
}
