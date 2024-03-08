// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"errors"
	"net/netip"
	"reflect"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/mitchellh/mapstructure"

	"akvorado/common/helpers"
	"akvorado/common/helpers/bimap"
	"akvorado/inlet/metadata/provider"
)

// Configuration describes the configuration for the SNMP client
type Configuration struct {
	// PollerRetries tell how many time a poller should retry before giving up
	PollerRetries int `validate:"min=0"`
	// PollerTimeout tell how much time a poller should wait for an answer
	PollerTimeout time.Duration `validate:"min=100ms"`

	// Communities is a mapping from exporter IPs to SNMPv2 communities
	Communities *helpers.SubnetMap[[]string]
	// SecurityParameters is a mapping from exporter IPs to SNMPv3 security parameters
	SecurityParameters *helpers.SubnetMap[SecurityParameters] `validate:"omitempty,dive"`
	// Agents is a mapping from exporter IPs to SNMP agent IP
	Agents map[netip.Addr]netip.Addr
	// Ports is a mapping from exporter IPs to SNMP port
	Ports *helpers.SubnetMap[uint16]
}

// SecurityParameters describes SNMPv3 USM security parameters.
type SecurityParameters struct {
	UserName                 string       `validate:"required"`
	AuthenticationProtocol   AuthProtocol `validate:"required_with=PrivacyProtocol"`
	AuthenticationPassphrase string       `validate:"required_with=AuthenticationProtocol"`
	PrivacyProtocol          PrivProtocol
	PrivacyPassphrase        string `validate:"required_with=PrivacyProtocol"`
	ContextName              string
}

// DefaultConfiguration represents the default configuration for the SNMP client.
func DefaultConfiguration() provider.Configuration {
	return Configuration{
		PollerRetries: 1,
		PollerTimeout: time.Second,

		Communities: helpers.MustNewSubnetMap(map[string][]string{
			"::/0": {"public"},
		}),
		SecurityParameters: helpers.MustNewSubnetMap(map[string]SecurityParameters{}),
		Ports: helpers.MustNewSubnetMap(map[string]uint16{
			"::/0": 161,
		}),
	}
}

// ConfigurationUnmarshallerHook normalize SNMP configuration:
//   - append default-community to communities (as ::/0)
//   - ensure we have a default value for communities
func ConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeOf(Configuration{}) {
			return from.Interface(), nil
		}

		// default-community → communities
		var defaultKey, mapKey *reflect.Value
		fromMap := from.MapKeys()
		for i, k := range fromMap {
			k = helpers.ElemOrIdentity(k)
			if k.Kind() != reflect.String {
				return from.Interface(), nil
			}
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
			} else if communities.Kind() == reflect.String {
				return nil, errors.New("do not provide default-community when using communities")
			} else {
				communities.SetMapIndex(reflect.ValueOf("::/0"), from.MapIndex(*defaultKey))
			}
		} else {
			// communities should contain ::/0
			if mapKey == nil {
				from.SetMapIndex(reflect.ValueOf("communities"), reflect.ValueOf("public"))
			} else if communities.Kind() != reflect.String && !communities.MapIndex(reflect.ValueOf("::/0")).IsValid() {
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

var authProtocolMap = bimap.New(map[AuthProtocol]string{
	AuthProtocol(gosnmp.NoAuth): "",
	AuthProtocol(gosnmp.MD5):    "MD5",
	AuthProtocol(gosnmp.SHA):    "SHA",
	AuthProtocol(gosnmp.SHA224): "SHA224",
	AuthProtocol(gosnmp.SHA256): "SHA256",
	AuthProtocol(gosnmp.SHA384): "SHA384",
	AuthProtocol(gosnmp.SHA512): "SHA512",
})

// UnmarshalText parses a SNMPv3 authentication protocol
func (ap *AuthProtocol) UnmarshalText(text []byte) error {
	protocol, ok := authProtocolMap.LoadKey(strings.ToUpper(string(text)))
	if !ok {
		return errors.New("unknown auth protocol")
	}
	*ap = AuthProtocol(protocol)
	return nil
}

// String turns a SNMPv3 authentication protocol to a string
func (ap AuthProtocol) String() string {
	protocol, ok := authProtocolMap.LoadValue(ap)
	if !ok {
		return ""
	}
	return protocol
}

// MarshalText turns a SNMPv3 authentication protocol to a string
func (ap AuthProtocol) MarshalText() ([]byte, error) {
	return []byte(ap.String()), nil
}

// PrivProtocol represents a SNMPv3 privacy protocol
type PrivProtocol gosnmp.SnmpV3PrivProtocol

var privProtocolMap = bimap.New(map[PrivProtocol]string{
	PrivProtocol(gosnmp.NoPriv):  "",
	PrivProtocol(gosnmp.DES):     "DES",
	PrivProtocol(gosnmp.AES):     "AES",
	PrivProtocol(gosnmp.AES192):  "AES192",
	PrivProtocol(gosnmp.AES256):  "AES256",
	PrivProtocol(gosnmp.AES192C): "AES192C",
	PrivProtocol(gosnmp.AES256C): "AES256C",
})

// UnmarshalText parses a SNMPv3 privacy protocol
func (pp *PrivProtocol) UnmarshalText(text []byte) error {
	protocol, ok := privProtocolMap.LoadKey(strings.ToUpper(string(text)))
	if !ok {
		return errors.New("unknown privacy protocol")
	}
	*pp = PrivProtocol(protocol)
	return nil
}

// String turns a SNMPv3 privacy protocol to a string
func (pp PrivProtocol) String() string {
	protocol, ok := privProtocolMap.LoadValue(pp)
	if !ok {
		return ""
	}
	return protocol
}

// MarshalText turns a SNMPv3 privacy protocol to a string
func (pp PrivProtocol) MarshalText() ([]byte, error) {
	return []byte(pp.String()), nil
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(ConfigurationUnmarshallerHook())
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[[]string]())
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[SecurityParameters]())
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[uint16]())
	helpers.RegisterSubnetMapValidation[SecurityParameters]()
	helpers.RegisterSubnetMapValidation[uint16]()
}
