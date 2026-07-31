// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package networks

import (
	"hash/maphash"
	"reflect"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/remotedatasource"

	"github.com/go-viper/mapstructure/v2"
)

// Configuration describes the configuration for the networks component.
type Configuration struct {
	// Networks is a mapping from IP networks to attributes.
	Networks *helpers.SubnetMap[NetworkAttributes] `validate:"omitempty,dive"`
	// NetworkSources defines a set of remote network definitions.
	NetworkSources map[string]remotedatasource.Source `validate:"dive"`
	// NetworkSourcesTimeout tells how long to wait for network sources to be ready.
	NetworkSourcesTimeout time.Duration `validate:"min=0"`
}

// DefaultConfiguration represents the default configuration for the networks component.
func DefaultConfiguration() Configuration {
	return Configuration{
		NetworkSourcesTimeout: 10 * time.Second,
	}
}

// NetworkAttributes is a set of attributes attached to a network.
type NetworkAttributes struct {
	// Name is a name attached to the network. May be unique or not.
	Name string
	// Role is a role attached to the network (server, customer).
	Role string
	// Site is the site of the network (ams5, pa3).
	Site string
	// Region is the region of the network (eu-west-1, us-east-3).
	Region string
	// City is the administrative city where the prefix is located (Paris, London).
	City string
	// State is the first administrative sub-division of the country (Ile-de-france, Alabama)
	State string
	// Country is the two-letter country code of the network (FR, IT). Longer
	// values are truncated when sent to ClickHouse.
	Country string
	// Tenant is a tenant for the network.
	Tenant string
	// ASN is the AS number associated to the network.
	ASN uint32
}

// attributesHashSeed is the seed used to hash the network attributes.
var attributesHashSeed = maphash.MakeSeed()

// Hash returns a hash of the network attributes, to intern them.
func (na NetworkAttributes) Hash() uint64 {
	hash := uint64(na.ASN)
	for _, value := range [...]string{
		na.Name, na.Role, na.Site, na.Region,
		na.City, na.State, na.Country, na.Tenant,
	} {
		hash = hash*31 + maphash.String(attributesHashSeed, value)
	}
	return hash
}

// Equal tells if two sets of network attributes are the same.
func (na NetworkAttributes) Equal(other NetworkAttributes) bool {
	return na == other
}

// NetworkAttributesUnmarshallerHook decodes network attributes. It
// also accepts a string instead of attributes for backward
// compatibility.
func NetworkAttributesUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (any, error) {
		from = helpers.ElemOrIdentity(from)
		to = helpers.ElemOrIdentity(to)
		if to.Type() != reflect.TypeFor[NetworkAttributes]() {
			return from.Interface(), nil
		}
		if from.Kind() == reflect.String {
			return NetworkAttributes{Name: from.String()}, nil
		}
		return from.Interface(), nil
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[NetworkAttributes]())
	helpers.RegisterMapstructureUnmarshallerHook(NetworkAttributesUnmarshallerHook())
	helpers.RegisterSubnetMapValidation[NetworkAttributes]()
}
