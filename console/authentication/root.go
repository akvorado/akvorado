package authentication

import "akvorado/common/reporter"

// Component represents the authentication compomenent.
type Component struct {
	r      *reporter.Reporter
	config Configuration
}

// New creates a new authentication component.
func New(r *reporter.Reporter, configuration Configuration) (*Component, error) {
	c := Component{
		r:      r,
		config: configuration,
	}

	return &c, nil
}
