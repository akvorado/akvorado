// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package database handles connection to a persistent database to
// save console settings.
package database

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "modernc.org/sqlite" // SQLite driver (no cgo)

	"akvorado/common/reporter"
)

// Component represents the database compomenent.
type Component struct {
	r      *reporter.Reporter
	config Configuration

	db *bun.DB
}

// New creates a new database component.
func New(r *reporter.Reporter, configuration Configuration) (*Component, error) {
	c := Component{
		r:      r,
		config: configuration,
	}
	return &c, nil
}

// Start starts the database component
func (c *Component) Start() error {
	c.r.Info().Msg("starting database component")
	switch c.config.Driver {
	case "sqlite":
		sqldb, err := sql.Open("sqlite", c.config.DSN)
		if err != nil {
			return fmt.Errorf("unable to open sqlite database: %w", err)
		}
		c.db = bun.NewDB(sqldb, sqlitedialect.New())
	case "postgresql":
		sqldb, err := sql.Open("pgx", c.config.DSN)
		if err != nil {
			return fmt.Errorf("unable to open PostgreSQL database: %w", err)
		}
		c.db = bun.NewDB(sqldb, pgdialect.New())
	case "mysql":
		sqldb, err := sql.Open("mysql", c.config.DSN)
		if err != nil {
			return fmt.Errorf("unable to open MySQL database: %w", err)
		}
		c.db = bun.NewDB(sqldb, mysqldialect.New())
	default:
		return fmt.Errorf("%q is not a supporter driver", c.config.Driver)
	}
	c.db.AddQueryHook(newQueryHook(c.r))

	ctx := context.Background()
	if _, err := c.db.NewCreateTable().
		Model((*SavedFilter)(nil)).
		IfNotExists().
		Exec(ctx); err != nil {
		return fmt.Errorf("cannot migrate database: %w", err)
	}
	if _, err := c.db.NewCreateIndex().
		Model((*SavedFilter)(nil)).
		Index("idx_saved_filters_user").
		Column("user").
		IfNotExists().
		Exec(ctx); err != nil {
		return fmt.Errorf("cannot migrate database: %w", err)
	}
	return c.populate()
}

// Stop stops the database component.
func (c *Component) Stop() error {
	defer c.r.Info().Msg("database component stopped")
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}
