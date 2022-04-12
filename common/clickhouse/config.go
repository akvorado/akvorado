package clickhouse

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// Configuration defines how we connect to a Clickhouse database
type Configuration struct {
	// Servers define the list of clickhouse servers to connect to (with ports)
	Servers []string
	// Database defines the database to use
	Database string
	// Username defines the username to use for authentication
	Username string
	// Password defines the password to use for authentication
	Password string
	// MaxOpenConns tells how many parallel connections to ClickHouse we want
	MaxOpenConns int
	// DialTimeout tells how much time to wait when connecting to ClickHouse
	DialTimeout time.Duration
}

// DefaultConfiguration represents the default configuration for connecting to Clickhouse
func DefaultConfiguration() Configuration {
	return Configuration{
		Servers:      []string{"127.0.0.1:9000"},
		Database:     "default",
		Username:     "default",
		MaxOpenConns: 10,
		DialTimeout:  5 * time.Second,
	}
}

// Open create a new
func (config Configuration) Open(ctx context.Context) (clickhouse.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: config.Servers,
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: config.Password,
		},
		Compression:     &clickhouse.Compression{clickhouse.CompressionLZ4},
		DialTimeout:     config.DialTimeout,
		MaxOpenConns:    config.MaxOpenConns,
		MaxIdleConns:    config.MaxOpenConns/2 + 1,
		ConnMaxLifetime: time.Hour,
	})
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, err
	}
	return conn, nil
}
