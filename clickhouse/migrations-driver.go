package clickhouse

import (
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2" // clickhouse driver for database/sql
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/atomic"
)

// Code below is a trimmed version of
// https://github.com/golang-migrate/migrate/blob/master/database/clickhouse/clickhouse.go
// The goal is to make it work with v2 driver. I have dropped
// clustering and hard-coded the database name.

func init() {
	database.Register("clickhouse", &ClickHouse{})
}

// ClickHouse struct represents the migrate ClickHouse driver
type ClickHouse struct {
	conn     *sql.DB
	isLocked atomic.Bool
}

// Open opens the connection to ClickHouse using the provided DSN
func (ch *ClickHouse) Open(dsn string) (database.Driver, error) {
	conn, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, err
	}
	ch = &ClickHouse{conn: conn}

	if err = ch.Lock(); err != nil {
		return nil, err
	}

	defer func() {
		if e := ch.Unlock(); e != nil {
			err = multierror.Append(err, e)
		}
	}()

	var (
		table string
		query = "SHOW TABLES LIKE 'schema_migrations'"
	)

	// Check if migration table exists
	if err := ch.conn.QueryRow(query).Scan(&table); err != nil {
		if err != sql.ErrNoRows {
			return nil, &database.Error{
				OrigErr: err,
				Err:     "cannot get list of tables",
				Query:   []byte(query),
			}
		}
	} else {
		return ch, nil
	}

	// If not, create
	query = `
          CREATE TABLE schema_migrations (
            version    Int64,
            dirty      UInt8,
            sequence   UInt64
          ) Engine=TinyLog`
	if _, err := ch.conn.Exec(query); err != nil {
		return nil, &database.Error{
			OrigErr: err,
			Err:     "cannot create schema migrations table",
			Query:   []byte(query),
		}
	}

	return ch, nil
}

// Run executes the provided migration
func (ch *ClickHouse) Run(r io.Reader) error {
	migration, err := ioutil.ReadAll(r)
	if err != nil {
		return fmt.Errorf("unable to read migration: %w", err)
	}
	if _, err := ch.conn.Exec(string(migration)); err != nil {
		return database.Error{OrigErr: err, Err: "migration failed", Query: migration}
	}
	return nil
}

// Version returns the current version.
func (ch *ClickHouse) Version() (int, bool, error) {
	var (
		version int
		dirty   uint8
	)
	query := "SELECT version, dirty FROM `schema_migrations` ORDER BY sequence DESC LIMIT 1"
	if err := ch.conn.QueryRow(query).Scan(&version, &dirty); err != nil {
		if err == sql.ErrNoRows {
			return database.NilVersion, false, nil
		}
		return 0, false, &database.Error{OrigErr: err, Err: "cannot get last version", Query: []byte(query)}
	}
	return version, dirty == 1, nil
}

// SetVersion updates the current version.
func (ch *ClickHouse) SetVersion(version int, dirty bool) error {
	bool := func(v bool) uint8 {
		if v {
			return 1
		}
		return 0
	}
	tx, err := ch.conn.Begin()
	if err != nil {
		return err
	}

	query := "INSERT INTO `schema_migrations` (version, dirty, sequence) VALUES"
	batch, err := tx.Prepare(query)
	if err != nil {
		return database.Error{OrigErr: err, Err: "cannot update schema migrations table", Query: []byte(query)}
	}
	if _, err := batch.Exec(int64(version), bool(dirty), uint64(time.Now().UnixNano())); err != nil {
		return &database.Error{OrigErr: err, Query: []byte(query)}
	}

	return tx.Commit()
}

// Drop should have removed the table. Not used.
func (ch *ClickHouse) Drop() (err error) {
	panic("unused")
}

// Lock locks the ClickHouse database.
func (ch *ClickHouse) Lock() error {
	if !ch.isLocked.CAS(false, true) {
		return database.ErrLocked
	}

	return nil
}

// Unlock unlocks the ClickHouse database.
func (ch *ClickHouse) Unlock() error {
	if !ch.isLocked.CAS(true, false) {
		return database.ErrNotLocked
	}

	return nil
}

// Close close the connection to ClickHouse.
func (ch *ClickHouse) Close() error {
	return ch.conn.Close()
}
