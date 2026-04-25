// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

// SavedFilter represents a saved filter in database.
type SavedFilter struct {
	bun.BaseModel `json:"-"`

	ID          uint64 `bun:",pk,autoincrement" json:"id"`
	User        string `json:"user"`
	Shared      bool   `json:"shared"`
	Description string `json:"description" validate:"required"`
	Content     string `json:"content" validate:"required"`
}

// To populate a few filters:
// http 127.0.0.1:8080/api/v0/console/filter/saved shared:=true description="ASN/To Iliad" content="InIfBoundary=external AND DstAS IN (AS12322, AS51207, AS29447)" Remote-User:spiderman
// http 127.0.0.1:8080/api/v0/console/filter/saved shared:=true description="ASN/From Google" content="InIfBoundary=external AND DstAS IN (AS15169, AS36040)" Remote-User:donald
// http 127.0.0.1:8080/api/v0/console/filter/saved shared:=true description="ASN/From Netflix" content="InIfBoundary=external AND (DstAS = AS2906 OR InIfProvider = 'netflix')" Remote-User:alfred

// CreateSavedFilter creates a new saved filter in database.
func (c *Component) CreateSavedFilter(ctx context.Context, f SavedFilter) error {
	f.ID = 0
	if _, err := c.db.NewInsert().Model(&f).Exec(ctx); err != nil {
		return fmt.Errorf("unable to create new saved filter: %w", err)
	}
	return nil
}

// ListSavedFilters list all saved filters for the provided user
func (c *Component) ListSavedFilters(ctx context.Context, user string) ([]SavedFilter, error) {
	results := []SavedFilter{}
	if err := c.db.NewSelect().
		Model(&results).
		Where("? = ?", bun.Ident("user"), user).
		WhereOr("? = ?", bun.Ident("shared"), true).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("unable to retrieve saved filters: %w", err)
	}
	return results, nil
}

// DeleteSavedFilter deletes the saved filter matching f.ID. If f.User is set,
// the filter must also belong to that user.
func (c *Component) DeleteSavedFilter(ctx context.Context, f SavedFilter) error {
	q := c.db.NewDelete().
		Model((*SavedFilter)(nil)).
		Where("? = ?", bun.Ident("id"), f.ID)
	if f.User != "" {
		q = q.Where("? = ?", bun.Ident("user"), f.User)
	}
	res, err := q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("cannot delete saved filter: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("cannot delete saved filter: %w", err)
	}
	if rows == 0 {
		return errors.New("no matching saved filter to delete")
	}
	return nil
}

const systemUser = "__system"

// Populate populates the database with the builtin filters.
func (c *Component) populate() error {
	ctx := context.Background()

	// Add new filters
	for _, filter := range c.config.SavedFilters {
		c.r.Debug().Msgf("add builtin filter %q", filter.Description)
		var existing SavedFilter
		err := c.db.NewSelect().Model(&existing).
			Where("? = ?", bun.Ident("user"), systemUser).
			Where("? = ?", bun.Ident("description"), filter.Description).
			Where("? = ?", bun.Ident("content"), filter.Content).
			Limit(1).
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			savedFilter := SavedFilter{
				User:        systemUser,
				Shared:      true,
				Description: filter.Description,
				Content:     filter.Content,
			}
			if _, err := c.db.NewInsert().Model(&savedFilter).Exec(ctx); err != nil {
				return fmt.Errorf("unable add builtin filter: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("cannot lookup builtin filter: %w", err)
		}
	}

	// Remove old filters
	var results []SavedFilter
	if err := c.db.NewSelect().
		Model(&results).
		Where("? = ?", bun.Ident("user"), systemUser).
		Where("? = ?", bun.Ident("shared"), true).
		Scan(ctx); err != nil {
		return fmt.Errorf("cannot get existing builtin filters: %w", err)
	}
outer:
	for _, result := range results {
		for _, filter := range c.config.SavedFilters {
			if filter.Description == result.Description && filter.Content == result.Content {
				continue outer
			}
		}
		c.r.Info().Msgf("remove old builtin filter %q", result.Description)
		if _, err := c.db.NewDelete().
			Model((*SavedFilter)(nil)).
			Where("? = ?", bun.Ident("id"), result.ID).
			Exec(ctx); err != nil {
			return fmt.Errorf("cannot delete old builtin filter: %w", err)
		}
	}

	return nil
}
