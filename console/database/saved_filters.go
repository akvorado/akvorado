package database

import (
	"context"
	"errors"
	"fmt"
)

// SavedFilter represents a saved filter in database.
type SavedFilter struct {
	ID          uint   `json:"id"`
	User        string `gorm:"index" json:"user"`
	Folder      string `json:"folder"`
	Description string `json:"description" binding:"required"`
	Content     string `json:"content" binding:"required"`
}

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
	result := c.db.WithContext(ctx).Where(&SavedFilter{User: user}).Find(&results)
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
