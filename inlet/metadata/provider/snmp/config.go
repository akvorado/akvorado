// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"net/netip"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gosnmp/gosnmp"
	"github.com/mitchellh/mapstructure"

	"akvorado/common/helpers"
	"akvorado/inlet/metadata/provider"
)

// Configuration describes the configuration for the SNMP client
type Configuration struct {
	// PollerRetries tell how many time a poller should retry before giving up
	PollerRetries int `validate:"min=0"`
	// PollerTimeout tell how much time a poller should wait for an answer
	PollerTimeout time.Duration `validate:"min=100ms"`

	// Credentials is a mapping from exporter IPs to credentials
	Credentials *helpers.SubnetMap[Credentials] `validate:"omitempty,dive"`
	// Agents is a mapping from exporter IPs to SNMP agent IP
	Agents map[netip.Addr]netip.Addr
	// Ports is a mapping from exporter IPs to SNMP port
	Ports *helpers.SubnetMap[uint16]
}

// Credentials describes credentials for SNMP (both SNMPv2 and SNMPv3 USM security parameters).
type Credentials struct {
	// SNMPv2
	Communities []string `yaml:",omitempty" validate:"excluded_with=UserName,required_without=UserName,omitempty,dive,required"`

	// SNMPv3
	UserName                 string       `yaml:",omitempty" validate:"excluded_with=Communities,required_without=Communities"`
	AuthenticationProtocol   AuthProtocol `yaml:",omitempty" validate:"excluded_with=Communities,required_with=PrivacyProtocol"`
	AuthenticationPassphrase string       `yaml:",omitempty" validate:"excluded_with=Communities,required_with=AuthenticationProtocol"`
	PrivacyProtocol          PrivProtocol `yaml:",omitempty" validate:"excluded_with=Communities"`
	PrivacyPassphrase        string       `yaml:",omitempty" validate:"excluded_with=Communities,required_with=PrivacyProtocol"`
	ContextName              string       `yaml:",omitempty" validate:"excluded_with=Communities"`
}

// DefaultConfiguration represents the default configuration for the SNMP client.
func DefaultConfiguration() provider.Configuration {
	return Configuration{
		PollerRetries: 1,
		PollerTimeout: time.Second,

		Credentials: helpers.MustNewSubnetMap(map[string]Credentials{
			"::/0": {
				Communities: []string{"public"},
			},
		}),
		Ports: helpers.MustNewSubnetMap(map[string]uint16{
			"::/0": 161,
		}),
	}
}

// ConfigurationUnmarshallerHook normalize SNMP configuration:
//   - convert default-community to credentials (as ::/0)
//   - merge security parameters and communities into credentials
func ConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeOf(Configuration{}) {
			return from.Interface(), nil
		}

		// default-community + security-parameters + communities → credentials
		var defaultCommunityKey, communitiesKey, securityParametersKey, credentialsKey *reflect.Value
		fromMap := from.MapKeys()
		for i, k := range fromMap {
			k = helpers.ElemOrIdentity(k)
			if k.Kind() != reflect.String {
				return from.Interface(), nil
			}
			if helpers.MapStructureMatchName(k.String(), "DefaultCommunity") {
				defaultCommunityKey = &fromMap[i]
			} else if helpers.MapStructureMatchName(k.String(), "Communities") {
				communitiesKey = &fromMap[i]
			} else if helpers.MapStructureMatchName(k.String(), "SecurityParameters") {
				securityParametersKey = &fromMap[i]
			} else if helpers.MapStructureMatchName(k.String(), "Credentials") {
				credentialsKey = &fromMap[i]
			}
		}
		var credentials reflect.Value
		if credentialsKey != nil {
			credentials = helpers.ElemOrIdentity(from.MapIndex(*credentialsKey))
		} else {
			credentials = reflect.ValueOf(gin.H{
				"::/0": gin.H{"communities": "public"},
			})
			from.SetMapIndex(reflect.ValueOf("credentials"), credentials)
		}
		if !helpers.LooksLikeSubnetMap(credentials) {
			credentials = reflect.ValueOf(gin.H{
				"::/0": credentials.Interface(),
			})
			from.SetMapIndex(reflect.ValueOf("credentials"), credentials)
		}
		// Convert default-community
		if defaultCommunityKey != nil && !helpers.ElemOrIdentity(from.MapIndex(*defaultCommunityKey)).IsZero() {
			credentials.SetMapIndex(reflect.ValueOf("::/0"), from.MapIndex(*defaultCommunityKey))
		}
		// Merge security-parameters and communities into credentials.
		if communitiesKey != nil {
			communitiesValue := helpers.ElemOrIdentity(from.MapIndex(*communitiesKey))
			if !communitiesValue.IsZero() {
				if !helpers.LooksLikeSubnetMap(communitiesValue) {
					credentials.SetMapIndex(reflect.ValueOf("::/0"), communitiesValue)
				} else {
					for _, key := range communitiesValue.MapKeys() {
						credentials.SetMapIndex(key, communitiesValue.MapIndex(key))
					}
				}
			}
		}
		if securityParametersKey != nil {
			securityParametersValue := helpers.ElemOrIdentity(from.MapIndex(*securityParametersKey))
			if !securityParametersValue.IsZero() {
				if !helpers.LooksLikeSubnetMap(securityParametersValue) {
					credentials.SetMapIndex(reflect.ValueOf("::/0"), securityParametersValue)
				} else {
					for _, key := range securityParametersValue.MapKeys() {
						credentials.SetMapIndex(key, securityParametersValue.MapIndex(key))
					}
				}
			}
		}
		// If any credential value is a string, assume this is a community
		for _, key := range credentials.MapKeys() {
			value := helpers.ElemOrIdentity(credentials.MapIndex(key))
			if value.Kind() == reflect.String || value.Kind() == reflect.Slice {
				credentials.SetMapIndex(key, reflect.ValueOf(gin.H{
					"communities": value.Interface(),
				}))
			}
		}

		if defaultCommunityKey != nil {
			from.SetMapIndex(*defaultCommunityKey, reflect.Value{})
		}
		if communitiesKey != nil {
			from.SetMapIndex(*communitiesKey, reflect.Value{})
		}
		if securityParametersKey != nil {
			from.SetMapIndex(*securityParametersKey, reflect.Value{})
		}
		return from.Interface(), nil
	}
}

// AuthProtocol represents a SNMPv3 authentication protocol
type AuthProtocol gosnmp.SnmpV3AuthProtocol

const (
	// AuthProtocolNone disables any authentication
	AuthProtocolNone AuthProtocol = AuthProtocol(gosnmp.NoAuth)
	// AuthProtocolMD5 uses MD5 authentication
	AuthProtocolMD5 AuthProtocol = AuthProtocol(gosnmp.MD5)
	// AuthProtocolSHA uses SHA authentication
	AuthProtocolSHA AuthProtocol = AuthProtocol(gosnmp.SHA)
	// AuthProtocolSHA224 uses SHA224 authentication
	AuthProtocolSHA224 AuthProtocol = AuthProtocol(gosnmp.SHA224)
	// AuthProtocolSHA256 uses SHA256 authentication
	AuthProtocolSHA256 AuthProtocol = AuthProtocol(gosnmp.SHA256)
	// AuthProtocolSHA384 uses SHA384 authentication
	AuthProtocolSHA384 AuthProtocol = AuthProtocol(gosnmp.SHA384)
	// AuthProtocolSHA512 uses SHA512 authentication
	AuthProtocolSHA512 AuthProtocol = AuthProtocol(gosnmp.SHA512)
)

// PrivProtocol represents a SNMPv3 privacy protocol
type PrivProtocol gosnmp.SnmpV3PrivProtocol

const (
	// PrivProtocolNone disables any encryption
	PrivProtocolNone PrivProtocol = PrivProtocol(gosnmp.NoPriv)
	// PrivProtocolDES uses DES for encryption
	PrivProtocolDES PrivProtocol = PrivProtocol(gosnmp.DES)
	// PrivProtocolAES uses AES for encryption
	PrivProtocolAES PrivProtocol = PrivProtocol(gosnmp.AES)
	// PrivProtocolAES192 uses AES192 for encryption
	PrivProtocolAES192 PrivProtocol = PrivProtocol(gosnmp.AES192)
	// PrivProtocolAES256 uses AES256 for encryption
	PrivProtocolAES256 PrivProtocol = PrivProtocol(gosnmp.AES256)
	// PrivProtocolAES192C uses AES192C for encryption
	PrivProtocolAES192C PrivProtocol = PrivProtocol(gosnmp.AES192C)
	// PrivProtocolAES256C uses AES256C for encryption
	PrivProtocolAES256C PrivProtocol = PrivProtocol(gosnmp.AES256C)
)

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(ConfigurationUnmarshallerHook())
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[[]string]())
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[Credentials]())
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[uint16]())
	helpers.RegisterSubnetMapValidation[Credentials]()
	helpers.RegisterSubnetMapValidation[uint16]()
}
