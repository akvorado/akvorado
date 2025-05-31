// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// SavedFilter represents a saved filter in database.
type SavedFilter struct {
	ID          uint64 `json:"id"`
	User        string `gorm:"index" json:"user"`
	Shared      bool   `json:"shared"`
	Description string `json:"description" binding:"required"`
	Content     string `json:"content" binding:"required"`
}

// To populate a few filters:
// http 127.0.0.1:8080/api/v0/console/filter/saved shared:=true description="ASN/To Iliad" content="InIfBoundary=external AND DstAS IN (AS12322, AS51207, AS29447)" Remote-User:spiderman
// http 127.0.0.1:8080/api/v0/console/filter/saved shared:=true description="ASN/From Google" content="InIfBoundary=external AND DstAS IN (AS15169, AS36040)" Remote-User:donald
// http 127.0.0.1:8080/api/v0/console/filter/saved shared:=true description="ASN/From Netflix" content="InIfBoundary=external AND (DstAS = AS2906 OR InIfProvider = 'netflix')" Remote-User:alfred

// CreateSavedFilter creates a new saved filter in database.
func (c *Component) CreateSavedFilter(ctx context.Context, f SavedFilter) error {
	f.ID = 0
	err := gorm.G[SavedFilter](c.db).Create(ctx, &f)
	if err != nil {
		return fmt.Errorf("unable to create new saved filter: %w", err)
	}
	return nil
}

// ListSavedFilters list all saved filters for the provided user
func (c *Component) ListSavedFilters(ctx context.Context, user string) ([]SavedFilter, error) {
	var results []SavedFilter
	results, err := gorm.G[SavedFilter](c.db).
		Where(SavedFilter{User: user}).
		Or(SavedFilter{Shared: true}).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve saved filters: %w", err)
	}
	return results, nil
}

// DeleteSavedFilter deletes the provided saved filter
func (c *Component) DeleteSavedFilter(ctx context.Context, f SavedFilter) error {
	rows, err := gorm.G[SavedFilter](c.db).Where(f).Delete(ctx)
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
	// Add new filters
	ctx := context.Background()
	db := gorm.G[SavedFilter](c.db)
	for _, filter := range c.config.SavedFilters {
		c.r.Debug().Msgf("add builtin filter %q", filter.Description)
		savedFilter := SavedFilter{
			User:        systemUser,
			Shared:      true,
			Description: filter.Description,
			Content:     filter.Content,
		}
		if _, err := db.Where(savedFilter).First(ctx); err == gorm.ErrRecordNotFound {
			err := db.Create(ctx, &savedFilter)
			if err != nil {
				return fmt.Errorf("unable add builtin filter: %w", err)
			}
		}
	}

	// Remove old filters
	results, err := db.Where(SavedFilter{User: systemUser, Shared: true}).Find(ctx)
	if err != nil {
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
		if _, err := db.Where(result).Delete(ctx); err != nil {
			return fmt.Errorf("cannot delete old builtin filter: %w", err)
		}
	}

	return nil
}
