// Package broker synchronizes the different internal services.
package broker

import (
	"encoding/json"
	netHTTP "net/http"
	"strings"
	"sync"

	"akvorado/common/http"
	"akvorado/common/reporter"
)

// Component represents the broker.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	config Configuration

	serviceConfigurationsLock sync.Mutex
	serviceConfigurations     map[ServiceType]interface{}
}

// Dependencies define the dependencies of the broker.
type Dependencies struct {
	HTTP *http.Component
}

// ServiceType describes the different internal services
type ServiceType string

var (
	// InletService represents the inlet service type
	InletService ServiceType = "inlet"
	// OrchestratorService represents the orchestrator service type
	OrchestratorService ServiceType = "orchestrator"
	// ConsoleService represents the console service type
	ConsoleService ServiceType = "console"
)

// New creates a new ClickHouse component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      r,
		d:      &dependencies,
		config: configuration,

		serviceConfigurations: map[ServiceType]interface{}{},
	}

	c.d.HTTP.AddHandler("/api/v0/orchestrator/broker/configuration/",
		netHTTP.HandlerFunc(c.configurationHandlerFunc))

	return &c, nil
}

// RegisterConfiguration registers the configuration for a service.
func (c *Component) RegisterConfiguration(service ServiceType, configuration interface{}) {
	c.serviceConfigurationsLock.Lock()
	c.serviceConfigurations[service] = configuration
	c.serviceConfigurationsLock.Unlock()
}

func (c *Component) configurationHandlerFunc(w netHTTP.ResponseWriter, req *netHTTP.Request) {
	service := strings.TrimPrefix(req.URL.Path, "/api/v0/orchestrator/broker/configuration")
	service = strings.Trim(service, "/")

	c.serviceConfigurationsLock.Lock()
	configuration, ok := c.serviceConfigurations[ServiceType(service)]
	c.serviceConfigurationsLock.Unlock()

	if !ok {
		netHTTP.Error(w, "Configuration not found.", netHTTP.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")
	encoder.Encode(configuration)
}
