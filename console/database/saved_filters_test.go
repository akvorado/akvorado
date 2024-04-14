// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func testSavedFilter(t *testing.T, c *Component) {
	// Create
	if err := c.CreateSavedFilter(context.Background(), SavedFilter{
		ID:          17,
		User:        "marty",
		Shared:      false,
		Description: "marty's filter",
		Content:     "SrcAS = 12322",
	}); err != nil {
		t.Fatalf("CreateSavedFilter() error:\n%+v", err)
	}
	if err := c.CreateSavedFilter(context.Background(), SavedFilter{
		User:        "judith",
		Shared:      true,
		Description: "judith's filter",
		Content:     "InIfBoundary = external",
	}); err != nil {
		t.Fatalf("CreateSavedFilter() error:\n%+v", err)
	}
	if err := c.CreateSavedFilter(context.Background(), SavedFilter{
		User:        "marty",
		Shared:      true,
		Description: "marty's second filter",
		Content:     "InIfBoundary = internal",
	}); err != nil {
		t.Fatalf("CreateSavedFilter() error:\n%+v", err)
	}

	// List
	got, err := c.ListSavedFilters(context.Background(), "marty")
	if err != nil {
		t.Fatalf("ListSavedFilters() error:\n%+v", err)
	}
	if diff := helpers.Diff(got, []SavedFilter{
		{
			ID:          1,
			User:        "marty",
			Shared:      false,
			Description: "marty's filter",
			Content:     "SrcAS = 12322",
		}, {
			ID:          2,
			User:        "judith",
			Shared:      true,
			Description: "judith's filter",
			Content:     "InIfBoundary = external",
		}, {
			ID:          3,
			User:        "marty",
			Shared:      true,
			Description: "marty's second filter",
			Content:     "InIfBoundary = internal",
		},
	}); diff != "" {
		t.Fatalf("ListSavedFilters() (-got, +want):\n%s", diff)
	}

	// Delete
	if err := c.DeleteSavedFilter(context.Background(), SavedFilter{ID: 1}); err != nil {
		t.Fatalf("DeleteSavedFilter() error:\n%+v", err)
	}
	got, _ = c.ListSavedFilters(context.Background(), "marty")
	if diff := helpers.Diff(got, []SavedFilter{
		{
			ID:          2,
			User:        "judith",
			Shared:      true,
			Description: "judith's filter",
			Content:     "InIfBoundary = external",
		}, {
			ID:          3,
			User:        "marty",
			Shared:      true,
			Description: "marty's second filter",
			Content:     "InIfBoundary = internal",
		},
	}); diff != "" {
		t.Fatalf("ListSavedFilters() (-got, +want):\n%s", diff)
	}
	if err := c.DeleteSavedFilter(context.Background(), SavedFilter{ID: 1}); err == nil {
		t.Fatal("DeleteSavedFilter() no error")
	}
}

func TestSavedFilterSqlite(t *testing.T) {
	r := reporter.NewMock(t)

	testSavedFilter(t, NewMock(t, r, DefaultConfiguration()))
}

func TestSavedFilterPostgres(t *testing.T) {
	server := helpers.CheckExternalService(t, "PostgreSQL", []string{"postgres:5432", "127.0.0.1:5432"})
	server, serverPort, err := net.SplitHostPort(server)
	if err != nil {
		t.Fatalf("failed to parse server:\n%+v", err)
	}

	r := reporter.NewMock(t)
	dsn := fmt.Sprintf(
		"host=%s port=%s user=akvorado password=akpass dbname=akvorado sslmode=disable",
		server, serverPort)
	c := NewMock(
		t,
		r,
		Configuration{
			Driver: "postgresql",
			DSN:    dsn,
		},
	)

	// clean database for future tests
	t.Cleanup(func() {
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatalf("sql.Open() error:\n%+v", err)
		}
		defer db.Close()
		if _, err := db.Exec(`
	DO $$ DECLARE
    r RECORD;
BEGIN
    -- if the schema you operate on is not "current", you will want to
    -- replace current_schema() in query with 'schematodeletetablesfrom'
    -- *and* update the generate 'DROP...' accordingly.
    FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = current_schema()) LOOP
        EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
    END LOOP;
END $$;
`); err != nil {
			t.Fatalf("db.Exec() error:\n%+v", err)
		}
	})

	testSavedFilter(t, c)
}

func TestSavedFilterMySQL(t *testing.T) {
	server := helpers.CheckExternalService(t, "MySQL", []string{"mysql:3306", "127.0.0.1:3306"})

	r := reporter.NewMock(t)
	dsn := fmt.Sprintf(
		"akvorado:akpass@tcp(%s)/akvorado?charset=utf8mb4&parseTime=True&loc=Local",
		server)
	c := NewMock(
		t,
		r,
		Configuration{
			Driver: "mysql",
			DSN:    dsn,
		},
	)

	// clean database for future tests
	t.Cleanup(func() {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			t.Fatalf("sql.Open() error:\n%+v", err)
		}
		defer db.Close()

		rows, err := db.Query("SHOW TABLES")
		if err != nil {
			t.Fatalf("db.Query() error\n%+v", err)
		}
		defer rows.Close()
		var tables []string
		for rows.Next() {
			var tableName string
			if err := rows.Scan(&tableName); err != nil {
				t.Fatalf("rows.Scan() error:\n%+v", err)
			}
			tables = append(tables, tableName)
		}
		for _, table := range tables {
			if _, err := db.Exec(fmt.Sprintf("DROP TABLE %s", table)); err != nil {
				t.Fatalf("db.Exec() error:\n%+v", err)
			}
		}
	})

	testSavedFilter(t, c)
}

func TestPopulateSavedFilters(t *testing.T) {
	config := DefaultConfiguration()
	config.SavedFilters = []BuiltinSavedFilter{
		{
			Description: "first filter",
			Content:     "content of first filter",
		}, {
			Description: "second filter",
			Content:     "content of second filter",
		},
	}
	r := reporter.NewMock(t)
	c := NewMock(t, r, config)

	got, _ := c.ListSavedFilters(context.Background(), "marty")
	if diff := helpers.Diff(got, []SavedFilter{
		{
			ID:          1,
			User:        "__system",
			Shared:      true,
			Description: "first filter",
			Content:     "content of first filter",
		}, {
			ID:          2,
			User:        "__system",
			Shared:      true,
			Description: "second filter",
			Content:     "content of second filter",
		},
	}); diff != "" {
		t.Fatalf("ListSavedFilters() (-got, +want):\n%s", diff)
	}

	c.config.SavedFilters = c.config.SavedFilters[1:]
	c.populate()
	got, _ = c.ListSavedFilters(context.Background(), "marty")
	if diff := helpers.Diff(got, []SavedFilter{
		{
			ID:          2,
			User:        "__system",
			Shared:      true,
			Description: "second filter",
			Content:     "content of second filter",
		},
	}); diff != "" {
		t.Fatalf("ListSavedFilters() (-got, +want):\n%s", diff)
	}
}
