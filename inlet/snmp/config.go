// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/mitchellh/mapstructure"

	"akvorado/common/helpers"
)

// Configuration describes the configuration for the SNMP client
type Configuration struct {
	// CacheDuration defines how long to keep cached entries without access
	CacheDuration time.Duration `validate:"min=1m"`
	// CacheRefresh defines how soon to refresh an existing cached entry
	CacheRefresh time.Duration `validate:"eq=0|min=1m,eq=0|gtefield=CacheDuration"`
	// CacheRefreshInterval defines the interval to check for expiration/refresh
	CacheCheckInterval time.Duration `validate:"ltefield=CacheRefresh"`
	// CachePersist defines a file to store cache and survive restarts
	CachePersistFile string
	// PollerRetries tell how many time a poller should retry before giving up
	PollerRetries int `validate:"min=0"`
	// PollerTimeout tell how much time a poller should wait for an answer
	PollerTimeout time.Duration
	// PollerCoalesce tells how many requests can be contained inside a single SNMP PDU
	PollerCoalesce int `validate:"min=0"`
	// Workers define the number of workers used to poll SNMP
	Workers int `validate:"min=1"`

	// Communities is a mapping from exporter IPs to SNMPv2 communities
	Communities *helpers.SubnetMap[string]
	// SecurityParameters is a mapping from exporter IPs to SNMPv3 security parameters
	SecurityParameters *helpers.SubnetMap[SecurityParameters]
}

// SecurityParameters describes SNMPv3 USM security parameters.
type SecurityParameters struct {
	UserName                 string       `validate:"required"`
	AuthenticationProtocol   AuthProtocol `validate:"required_with=PrivProtocol"`
	AuthenticationPassphrase string       `validate:"required_with=AuthenticationProtocol"`
	PrivacyProtocol          PrivProtocol
	PrivacyPassphrase        string `validate:"required_with=PrivacyProtocol"`
	ContextName              string
}

// DefaultConfiguration represents the default configuration for the SNMP client.
func DefaultConfiguration() Configuration {
	return Configuration{
		CacheDuration:      30 * time.Minute,
		CacheRefresh:       time.Hour,
		CacheCheckInterval: 2 * time.Minute,
		CachePersistFile:   "",
		PollerRetries:      1,
		PollerTimeout:      time.Second,
		PollerCoalesce:     10,
		Workers:            1,

		Communities: helpers.MustNewSubnetMap(map[string]string{
			"::/0": "public",
		}),
		SecurityParameters: helpers.MustNewSubnetMap(map[string]SecurityParameters{}),
	}
}

// ConfigurationUnmarshallerHook normalize SNMP configuration:
//   - append default-community to communities (as ::/0)
//   - ensure we have a default value for communities
func ConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.Map || from.IsNil() || from.Type().Key().Kind() != reflect.String || to.Type() != reflect.TypeOf(Configuration{}) {
			return from.Interface(), nil
		}

		// default-community → communities
		var defaultKey, mapKey *reflect.Value
		fromMap := from.MapKeys()
		for i, k := range fromMap {
			if helpers.MapStructureMatchName(k.String(), "DefaultCommunity") {
				defaultKey = &fromMap[i]
			} else if helpers.MapStructureMatchName(k.String(), "Communities") {
				mapKey = &fromMap[i]
			}
		}
		var communities reflect.Value
		if mapKey != nil {
			communities = helpers.ElemOrIdentity(from.MapIndex(*mapKey))
		}
		if defaultKey != nil && !helpers.ElemOrIdentity(from.MapIndex(*defaultKey)).IsZero() {
			if mapKey == nil {
				// Use the fact we can set the default value directly.
				from.SetMapIndex(reflect.ValueOf("communities"), from.MapIndex(*defaultKey))
			} else {
				communities.SetMapIndex(reflect.ValueOf("::/0"), from.MapIndex(*defaultKey))
			}
		} else {
			// default-community should contain ::/0
			if mapKey == nil {
				from.SetMapIndex(reflect.ValueOf("communities"), reflect.ValueOf("public"))
			} else if !communities.MapIndex(reflect.ValueOf("::/0")).IsValid() {
				communities.SetMapIndex(reflect.ValueOf("::/0"), reflect.ValueOf("public"))
			}
		}
		if defaultKey != nil {
			from.SetMapIndex(*defaultKey, reflect.Value{})
		}

		return from.Interface(), nil
	}
}

// AuthProtocol represents a SNMPv3 authentication protocol
type AuthProtocol gosnmp.SnmpV3AuthProtocol

// UnmarshalText parses a SNMPv3 authentication protocol
func (ap *AuthProtocol) UnmarshalText(text []byte) error {
	switch strings.ToUpper(string(text)) {
	case "":
		*ap = AuthProtocol(gosnmp.NoAuth)
	case "MD5":
		*ap = AuthProtocol(gosnmp.MD5)
	case "SHA":
		*ap = AuthProtocol(gosnmp.SHA)
	case "SHA224":
		*ap = AuthProtocol(gosnmp.SHA224)
	case "SHA256":
		*ap = AuthProtocol(gosnmp.SHA256)
	case "SHA384":
		*ap = AuthProtocol(gosnmp.SHA384)
	case "SHA512":
		*ap = AuthProtocol(gosnmp.SHA512)
	default:
		return errors.New("unknown auth protocol")
	}
	return nil
}

// String turns a SNMPv3 authentication protocol to a string
func (ap AuthProtocol) String() string {
	return gosnmp.SnmpV3AuthProtocol(ap).String()
}

// MarshalText turns a SNMPv3 authentication protocol to a string
func (ap AuthProtocol) MarshalText() ([]byte, error) {
	return []byte(ap.String()), nil
}

// PrivProtocol represents a SNMPv3 privacy protocol
type PrivProtocol gosnmp.SnmpV3PrivProtocol

// UnmarshalText parses a SNMPv3 privacy protocol
func (ap *PrivProtocol) UnmarshalText(text []byte) error {
	switch strings.ToUpper(string(text)) {
	case "":
		*ap = PrivProtocol(gosnmp.NoPriv)
	case "DES":
		*ap = PrivProtocol(gosnmp.DES)
	case "AES":
		*ap = PrivProtocol(gosnmp.AES)
	case "AES192":
		*ap = PrivProtocol(gosnmp.AES192)
	case "AES256":
		*ap = PrivProtocol(gosnmp.AES256)
	case "AES192C":
		*ap = PrivProtocol(gosnmp.AES192C)
	case "AES256C":
		*ap = PrivProtocol(gosnmp.AES256C)
	default:
		return errors.New("unknown priv protocol")
	}
	return nil
}

// String turns a SNMPv3 privacy protocol to a string
func (ap PrivProtocol) String() string {
	return gosnmp.SnmpV3PrivProtocol(ap).String()
}

// MarshalText turns a SNMPv3 privacy protocol to a string
func (ap PrivProtocol) MarshalText() ([]byte, error) {
	return []byte(ap.String()), nil
}

func init() {
	helpers.AddMapstructureUnmarshallerHook(ConfigurationUnmarshallerHook())
	helpers.AddMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[string]())
	helpers.AddMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[SecurityParameters]())
}
