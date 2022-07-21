// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Configuration describes the configuration for the console component.
type Configuration struct {
	// ServeLiveFS serve files from the filesystem instead of the embedded versions.
	ServeLiveFS bool `yaml:"-"`
	// Version is the version to display to the user.
	Version string `yaml:"-"`
}

// DefaultConfiguration represents the default configuration for the console component.
func DefaultConfiguration() Configuration {
	return Configuration{}
}

func (c *Component) configHandlerFunc(gc *gin.Context) {
	gc.JSON(http.StatusOK, gin.H{"version": c.config.Version})
}
