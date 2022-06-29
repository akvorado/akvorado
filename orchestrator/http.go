// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package orchestrator

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (c *Component) configurationHandlerFunc(gc *gin.Context) {
	service := gc.Param("service")
	indexStr := gc.Param("index")
	index, err := strconv.Atoi(indexStr)
	if indexStr != "" && err != nil {
		gc.JSON(http.StatusNotFound, gin.H{"message": "Invalid configuration index."})
		return
	}

	c.serviceLock.Lock()
	var configuration interface{}
	serviceConfigurations, ok := c.serviceConfigurations[ServiceType(service)]
	if ok {
		l := len(serviceConfigurations)
		switch {
		case l == 0:
			ok = false
		case index < l:
			configuration = serviceConfigurations[index]
		default:
			configuration = serviceConfigurations[0]
		}
	}
	c.serviceLock.Unlock()

	if !ok {
		gc.JSON(http.StatusNotFound, gin.H{"message": "Configuration not found."})
		return
	}
	gc.YAML(http.StatusOK, configuration)
}
