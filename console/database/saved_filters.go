// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"context"
	"errors"
	"fmt"
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
	result := c.db.WithContext(ctx).Omit("ID").Create(&f)
	if result.Error != nil {
		return fmt.Errorf("unable to create new saved filter: %w", result.Error)
	}
	return nil
}

// ListSavedFilters list all saved filters for the provided user
func (c *Component) ListSavedFilters(ctx context.Context, user string) ([]SavedFilter, error) {
	var results []SavedFilter
	result := c.db.WithContext(ctx).
		Where(&SavedFilter{User: user}).
		Or(&SavedFilter{Shared: true}).
		Find(&results)
	if result.Error != nil {
		return nil, fmt.Errorf("unable to retrieve saved filters: %w", result.Error)
	}
	return results, nil
}

// DeleteSavedFilter deletes the provided saved filter
func (c *Component) DeleteSavedFilter(ctx context.Context, f SavedFilter) error {
	result := c.db.WithContext(ctx).Where(&SavedFilter{User: f.User}).Delete(&f)
	if result.Error != nil {
		return fmt.Errorf("cannot delete saved filter: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("no matching saved filter to delete")
	}
	return nil
}

const systemUser = "__system"

// Populate populates the database with the builtin filters.
func (c *Component) populate() error {
	// Add new filters
	for _, filter := range c.config.SavedFilters {
		c.r.Debug().Msgf("add builtin filter %q", filter.Description)
		savedFilter := SavedFilter{
			User:        systemUser,
			Shared:      true,
			Description: filter.Description,
			Content:     filter.Content,
		}
		result := c.db.Where(savedFilter).FirstOrCreate(&savedFilter)
		if result.Error != nil {
			return fmt.Errorf("unable add builtin filter: %w", result.Error)
		}
	}

	// Remove old filters
	var results []SavedFilter
	result := c.db.Where(SavedFilter{User: systemUser, Shared: true}).Find(&results)
	if result.Error != nil {
		return fmt.Errorf("cannot get existing builtin filters: %w", result.Error)
	}
outer:
	for _, result := range results {
		for _, filter := range c.config.SavedFilters {
			if filter.Description == result.Description && filter.Content == result.Content {
				continue outer
			}
		}
		c.r.Info().Msgf("remove old builtin filter %q", result.Description)
		if result := c.db.Delete(&result); result.Error != nil {
			return fmt.Errorf("cannot delete old builtin filter: %w", result.Error)
		}
	}

	return nil
}
