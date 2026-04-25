// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package orchestrator

import (
	"net/http"
	"strconv"

	"akvorado/common/helpers"
	"akvorado/common/httpserver"
)

func (c *Component) configurationHandlerFunc(w http.ResponseWriter, req *http.Request) {
	service := req.PathValue("service")
	indexStr := req.PathValue("index")
	index, err := strconv.Atoi(indexStr)
	if indexStr != "" && err != nil {
		httpserver.WriteJSON(w, http.StatusNotFound, helpers.M{"message": "Invalid configuration index."})
		return
	}

	c.serviceLock.Lock()
	var configuration any
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
		httpserver.WriteJSON(w, http.StatusNotFound, helpers.M{"message": "Configuration not found."})
		return
	}
	httpserver.WriteYAML(w, http.StatusOK, configuration)
}
