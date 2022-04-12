//go:build !release

package clickhouse

import (
	"context"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"akvorado/common/helpers"
)

// SetupClickHouse configures a client to use for testing.
func SetupClickHouse(t *testing.T) (clickhouse.Conn, []string) {
	t.Helper()
	chServer := helpers.CheckExternalService(t, "ClickHouse", []string{"clickhouse", "localhost"}, "9000")
	config := DefaultConfiguration()
	config.Servers = []string{chServer}
	config.DialTimeout = 100 * time.Millisecond

	conn, err := config.Open(context.Background())
	if err != nil {
		t.Fatalf("Open() error:\n%+v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("Close() error:\n%+v", err)
		}
	})
	return conn, []string{chServer}
}
