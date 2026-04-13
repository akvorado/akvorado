// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package gnmi

import (
	"errors"
	"net/netip"
	"reflect"
	"time"

	"akvorado/common/helpers"
	"akvorado/outlet/metadata/provider"

	"github.com/go-viper/mapstructure/v2"
)

// Configuration describes the configuration for the gNMI client
type Configuration struct {
	// Timeout tells how much time to wait for an answer
	Timeout time.Duration `validate:"min=100ms"`
	// MinimalRefreshInterval tells how much time to wait at least between two refreshes
	MinimalRefreshInterval time.Duration `validate:"min=1s"`
	// Targets is a mapping from exporter IPs to gNMI target IP.
	Targets *helpers.SubnetMap[netip.Addr]
	// SetTarget is a mapping from exporter IPs to whatever set target name in gNMI path prefix
	SetTarget *helpers.SubnetMap[bool]
	// Ports is a mapping from exporter IPs to gNMI port.
	Ports *helpers.SubnetMap[uint16]
	// AuthenticationParameters is a mapping from exporter IPs to authentication configuration.
	AuthenticationParameters *helpers.SubnetMap[AuthenticationParameter] `validate:"omitempty,dive"`
	// Models describe the YANG models to use to query devices.
	Models []Model `validate:"min=1,dive"`
}

// AuthenticationParameter contains the configuration related to authentication to a target.
type AuthenticationParameter struct {
	// Username is the username to use to authenticate.
	Username string
	// Password is the password to use to authenticate.
	Password string `validate:"required_with=Username"`
	// TLS defines the TLS configuration for the gRPC connection.
	TLS helpers.TLSConfiguration
}

// Model defines a model to retrieve data.
type Model struct {
	Name               string        `validate:"required"`
	SystemNamePaths    []string      `validate:"min=1"`
	IfIndexPaths       []string      `validate:"min=1"`
	IfNameKeys         []string      `validate:"required_without=IfNamePaths"`
	IfNamePaths        []string      `validate:"required_without=IfNameKeys"`
	IfDescriptionPaths []string      `validate:"min=1"`
	IfSpeedPaths       []IfSpeedPath `validate:"min=1,dive"`
}

// IfSpeedPath defines a path for oper speed.
type IfSpeedPath struct {
	Path string          `validate:"required"`
	Unit IfSpeedPathUnit `validate:"required"`
}

// IfSpeedPathUnit defines an SASL algorithm
type IfSpeedPathUnit int

const (
	// SpeedBps means the speed is in bps
	SpeedBps IfSpeedPathUnit = iota + 1
	// SpeedMbps means the speed is in Mbps
	SpeedMbps
	// SpeedEthernet means the speed is in OC ETHERNET_SPEED
	SpeedEthernet
	// SpeedHuman means the speed is human-formatted (10G)
	SpeedHuman
)

// DefaultConfiguration represents the default configuration for the SNMP client.
func DefaultConfiguration() provider.Configuration {
	return Configuration{
		Timeout:                  time.Second,
		MinimalRefreshInterval:   time.Minute,
		Targets:                  helpers.MustNewSubnetMap(map[string]netip.Addr{}),
		SetTarget:                helpers.MustNewSubnetMap(map[string]bool{}),
		Ports:                    helpers.MustNewSubnetMap(map[string]uint16{"::/0": 9339}),
		AuthenticationParameters: helpers.MustNewSubnetMap(map[string]AuthenticationParameter{}),
		Models:                   DefaultModels(),
	}
}

// DefaultModels return the builtin list of models.
func DefaultModels() []Model {
	// No origin means the native one.
	return []Model{
		{
			Name:            "Nokia SR OS",
			SystemNamePaths: []string{"/configure/system/name"},
			IfIndexPaths: []string{
				"/state/port/if-index",
				"/state/lag/if-index",
			},
			IfNameKeys: []string{"port-id", "lag-name"},
			IfDescriptionPaths: []string{
				"/configure/port/description",
				"/configure/lag/description",
			},
			IfSpeedPaths: []IfSpeedPath{
				{"/state/port/ethernet/oper-speed", SpeedMbps},
				{"/state/lag/bandwidth", SpeedBps},
			},
		}, {
			Name:            "Nokia SR Linux",
			SystemNamePaths: []string{"/system/name/host-name"},
			IfIndexPaths: []string{
				"/interface/ifindex",
				"/interface/subinterface/ifindex",
			},
			IfNameKeys: []string{"name"},
			IfNamePaths: []string{
				"/interface/subinterface/name",
			},
			IfDescriptionPaths: []string{
				"/interface/description",
				"/interface/subinterface/description",
			},
			IfSpeedPaths: []IfSpeedPath{
				{"/interface/ethernet/port-speed", SpeedHuman},
				{"/interface/lag/lag-speed", SpeedBps},
			},
		}, {
			Name:            "OpenConfig",
			SystemNamePaths: []string{"/system/config/hostname"},
			IfIndexPaths: []string{
				"/interfaces/interface/state/ifindex",
				"/interfaces/interface/subinterfaces/subinterface/state/ifindex",
			},
			IfNameKeys: []string{"name"},
			IfNamePaths: []string{
				"/interfaces/interface/subinterfaces/subinterface/state/name",
			},
			IfDescriptionPaths: []string{
				"/interfaces/interface/state/description",
				"/interfaces/interface/subinterfaces/subinterface/state/description",
			},
			IfSpeedPaths: []IfSpeedPath{
				{"/interfaces/interface/aggregation/state/lag-speed", SpeedMbps},
				{"/interfaces/interface/ethernet/state/negotiated-port-speed", SpeedEthernet},
				{"/interfaces/interface/ethernet/state/port-speed", SpeedEthernet},
			},
		}, {
			Name:               "IETF",
			SystemNamePaths:    []string{"/system/hostname"},
			IfIndexPaths:       []string{"/interfaces/interface/if-index"},
			IfNamePaths:        []string{"/interfaces/interface/name"},
			IfDescriptionPaths: []string{"/interfaces/interface/description"},
			IfSpeedPaths: []IfSpeedPath{
				{"/interfaces/interface/speed", SpeedBps},
			},
		},
	}
}

// AuthenticationParameterUnmarshallerHook migrates old TLS configuration to new format:
//   - Insecure → TLS.Enable (inverted)
//   - SkipVerify → TLS.SkipVerify
//   - TLSCA → TLS.CAFile
//   - TLSCert → TLS.CertFile
//   - TLSKey → TLS.KeyFile
//   - If no Insecure field is present, TLS.Enable defaults to true
func AuthenticationParameterUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (any, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeFor[AuthenticationParameter]() {
			return from.Interface(), nil
		}

		var insecureKey, skipVerifyKey, tlsCAKey, tlsCertKey, tlsKeyKey, tlsKey *reflect.Value
		fromMap := from.MapKeys()
		for i, k := range fromMap {
			k = helpers.ElemOrIdentity(k)
			if k.Kind() != reflect.String {
				return from.Interface(), nil
			}
			switch {
			case helpers.MapStructureMatchName(k.String(), "Insecure"):
				insecureKey = &fromMap[i]
			case helpers.MapStructureMatchName(k.String(), "SkipVerify"):
				skipVerifyKey = &fromMap[i]
			case helpers.MapStructureMatchName(k.String(), "TLSCA"):
				tlsCAKey = &fromMap[i]
			case helpers.MapStructureMatchName(k.String(), "TLSCert"):
				tlsCertKey = &fromMap[i]
			case helpers.MapStructureMatchName(k.String(), "TLSKey"):
				tlsKeyKey = &fromMap[i]
			case helpers.MapStructureMatchName(k.String(), "TLS"):
				tlsKey = &fromMap[i]
			}
		}

		// If we have new TLS key and any old keys, that's an error
		if tlsKey != nil && (insecureKey != nil || skipVerifyKey != nil || tlsCAKey != nil || tlsCertKey != nil || tlsKeyKey != nil) {
			return nil, errors.New("cannot mix old TLS configuration (Insecure, SkipVerify, TLSCA, TLSCert, TLSKey) with new TLS configuration")
		}

		// If no old keys, we need to set default TLS.Enable
		if tlsKey != nil {
			tlsValue := helpers.ElemOrIdentity(from.MapIndex(*tlsKey))
			if tlsValue.Kind() == reflect.Map {
				// Check if Enable is already set
				var enableKey *reflect.Value
				for _, k := range tlsValue.MapKeys() {
					k = helpers.ElemOrIdentity(k)
					if k.Kind() == reflect.String && helpers.MapStructureMatchName(k.String(), "Enable") {
						enableKey = &k
						break
					}
				}
				if enableKey == nil {
					// Set Enable to true by default
					tlsValue.SetMapIndex(reflect.ValueOf("enable"), reflect.ValueOf(true))
				}
			}
			return from.Interface(), nil
		}

		// Migrate old configuration to new format
		tlsConfig := make(map[string]any)

		// Handle Insecure → TLS.Enable (inverted)
		if insecureKey != nil {
			insecureValue := helpers.ElemOrIdentity(from.MapIndex(*insecureKey))
			if insecureValue.Kind() == reflect.Bool {
				tlsConfig["enable"] = !insecureValue.Bool()
			}
			from.SetMapIndex(*insecureKey, reflect.Value{})
		} else {
			// Default to TLS enabled if Insecure not specified
			tlsConfig["enable"] = true
		}

		// Handle SkipVerify → TLS.SkipVerify
		if skipVerifyKey != nil {
			skipVerifyValue := helpers.ElemOrIdentity(from.MapIndex(*skipVerifyKey))
			if skipVerifyValue.Kind() == reflect.Bool {
				tlsConfig["skip-verify"] = skipVerifyValue.Bool()
			}
			from.SetMapIndex(*skipVerifyKey, reflect.Value{})
		}

		// Handle TLSCA → TLS.CAFile
		if tlsCAKey != nil {
			tlsCAValue := helpers.ElemOrIdentity(from.MapIndex(*tlsCAKey))
			if tlsCAValue.Kind() == reflect.String {
				tlsConfig["ca-file"] = tlsCAValue.String()
			}
			from.SetMapIndex(*tlsCAKey, reflect.Value{})
		}

		// Handle TLSCert → TLS.CertFile
		if tlsCertKey != nil {
			tlsCertValue := helpers.ElemOrIdentity(from.MapIndex(*tlsCertKey))
			if tlsCertValue.Kind() == reflect.String {
				tlsConfig["cert-file"] = tlsCertValue.String()
			}
			from.SetMapIndex(*tlsCertKey, reflect.Value{})
		}

		// Handle TLSKey → TLS.KeyFile
		if tlsKeyKey != nil {
			tlsKeyValue := helpers.ElemOrIdentity(from.MapIndex(*tlsKeyKey))
			if tlsKeyValue.Kind() == reflect.String {
				tlsConfig["key-file"] = tlsKeyValue.String()
			}
			from.SetMapIndex(*tlsKeyKey, reflect.Value{})
		}

		// Set the new TLS configuration
		from.SetMapIndex(reflect.ValueOf("tls"), reflect.ValueOf(tlsConfig))

		return from.Interface(), nil
	}
}

// ConfigurationUnmarshallerHook normalize gnmi configuration:
//   - replace an occurrence of "default" in the list of models with the list of default models.
func ConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (any, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeFor[Configuration]() {
			return from.Interface(), nil
		}

		var modelsKey *reflect.Value
		fromMap := from.MapKeys()
		for i, k := range fromMap {
			k = helpers.ElemOrIdentity(k)
			if k.Kind() != reflect.String {
				return from.Interface(), nil
			}
			if helpers.MapStructureMatchName(k.String(), "Models") {
				modelsKey = &fromMap[i]
			}
		}
		if modelsKey != nil {
			modelsValue := helpers.ElemOrIdentity(from.MapIndex(*modelsKey))
			if modelsValue.Kind() == reflect.Array || modelsValue.Kind() == reflect.Slice {
				for i := range modelsValue.Len() {
					val := helpers.ElemOrIdentity(modelsValue.Index(i))
					if val.Kind() == reflect.String && val.String() == "defaults" {
						// We need to replace this item with the default values.
						newValue := reflect.MakeSlice(reflect.SliceOf(reflect.TypeFor[any]()), 0, 0)
						for j := range modelsValue.Len() {
							if i != j {
								newValue = reflect.Append(newValue, modelsValue.Index(j))
							} else {
								defaults := DefaultModels()
								for k := range len(defaults) {
									newValue = reflect.Append(newValue, reflect.ValueOf(defaults[k]))
								}
							}
						}
						from.SetMapIndex(*modelsKey, newValue)
						break
					}
				}
			}
		}

		return from.Interface(), nil
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(AuthenticationParameterUnmarshallerHook())
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[bool]())
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[uint16]())
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[AuthenticationParameter]())
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[netip.Addr]())
	helpers.RegisterMapstructureUnmarshallerHook(ConfigurationUnmarshallerHook())
	helpers.RegisterSubnetMapValidation[bool]()
	helpers.RegisterSubnetMapValidation[uint16]()
	helpers.RegisterSubnetMapValidation[AuthenticationParameter]()
	helpers.RegisterSubnetMapValidation[netip.Addr]()
}
