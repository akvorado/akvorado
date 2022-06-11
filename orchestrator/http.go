package orchestrator

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (c *Component) configurationHandlerFunc(gc *gin.Context) {
	service := gc.Param("service")

	c.serviceLock.Lock()
	configuration, ok := c.serviceConfigurations[ServiceType(service)]
	c.serviceLock.Unlock()

	if !ok {
		gc.JSON(http.StatusNotFound, gin.H{"message": "Configuration not found."})
		return
	}
	gc.IndentedJSON(http.StatusOK, configuration)

	c.serviceLock.Lock()
	if c.registeredServices[ServiceType(service)] == nil {
		c.registeredServices[ServiceType(service)] = map[string]bool{}
	}
	c.registeredServices[ServiceType(service)][gc.ClientIP()] = true
	c.serviceLock.Unlock()
}
