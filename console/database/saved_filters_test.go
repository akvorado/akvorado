// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"context"
	"log"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/reporter"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
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

	testSavedFilter(t,  NewMock(t, r, DefaultConfiguration()))
}

func TestSavedFilterPostgres(t *testing.T) {
	r := reporter.NewMock(t)
	ctx := context.Background()

	dbName := "akvorado"
	dbUser := "akvorado"
	dbPassword := "akpass"

	postgresContainer, err := postgres.RunContainer(ctx,
			testcontainers.WithImage("docker.io/postgres:16-alpine"),
			postgres.WithDatabase(dbName),
			postgres.WithUsername(dbUser),
			postgres.WithPassword(dbPassword),
			testcontainers.WithWaitStrategy(
					wait.ForLog("database system is ready to accept connections").
							WithOccurrence(2).
							WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
			log.Fatalf("failed to start container: %s", err)
	}

	// Clean up the container
	defer func() {
			if err := postgresContainer.Terminate(ctx); err != nil {
					log.Fatalf("failed to terminate container: %s", err)
			}
	}()

	dsn, err := postgresContainer.ConnectionString(ctx)

  if err != nil {
		t.Fatalf("failed to get postgres connection string: %s", err)
	}

	c := NewMock(
		t,
		r,
		Configuration{
			Driver: "postgresql",
			DSN: dsn,
		},
	)

	testSavedFilter(t,  c)
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
