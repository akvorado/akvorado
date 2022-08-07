// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package database handles connection to a persistent database to
// save console settings.
package database

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"akvorado/common/reporter"
)

// Component represents the database compomenent.
type Component struct {
	r      *reporter.Reporter
	config Configuration

	db *gorm.DB
}

// New creates a new database component.
func New(r *reporter.Reporter, configuration Configuration) (*Component, error) {
	c := Component{
		r:      r,
		config: configuration,
	}
	switch c.config.Driver {
	case "sqlite":
		db, err := gorm.Open(sqlite.Open(c.config.DSN), &gorm.Config{
			Logger: &logger{r},
		})
		if err != nil {
			return nil, fmt.Errorf("unable to open database: %w", err)
		}
		c.db = db
	default:
		return nil, fmt.Errorf("%q is not a supporter driver", c.config.Driver)
	}
	return &c, nil
}

// Start starts the database component
func (c *Component) Start() error {
	c.r.Info().Msg("starting database component")
	if err := c.db.AutoMigrate(&SavedFilter{}); err != nil {
		return fmt.Errorf("cannot migrate database: %w", err)
	}
	return nil
}

// Stop stops the database component.
func (c *Component) Stop() error {
	defer c.r.Info().Msg("database component stopped")
	if c.db != nil {
		sqlDB, err := c.db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
