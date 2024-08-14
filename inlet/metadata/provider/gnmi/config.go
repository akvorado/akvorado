// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package gnmi

import (
	"errors"
	"net/netip"
	"reflect"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/helpers/bimap"
	"akvorado/inlet/metadata/provider"

	"github.com/mitchellh/mapstructure"
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
	// Insecure tells if the gRPC connection is in clear-text.
	Insecure bool
	// SkipVerify tells if we should skip certificate verification for the gRPC connection.
	SkipVerify bool
	// TLSCA sets the path towards the TLS certificate authority file.
	TLSCA string
	// TLSCert sets the path towards the TLS certificate file.
	TLSCert string
	// TLSKey sets the path towards the TLS key file.
	TLSKey string
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
	SpeedBits     IfSpeedPathUnit = iota + 1 // SpeedBits means the speed is in bps
	SpeedMegabits                            // SpeedMegabits means the speed is in Mbps
	SpeedEthernet                            // SpeedEthernet means the speed is in OC ETHERNET_SPEED
	SpeedHuman                               // SpeedHuman means the speed is human-formatted (10G)
)

var ifSpeedPathUnitMap = bimap.New(map[IfSpeedPathUnit]string{
	SpeedBits:     "bps",
	SpeedMegabits: "mbps",
	SpeedEthernet: "ethernet",
	SpeedHuman:    "human",
})

// MarshalText turns a speed unit to text
func (sa IfSpeedPathUnit) MarshalText() ([]byte, error) {
	got, ok := ifSpeedPathUnitMap.LoadValue(sa)
	if ok {
		return []byte(got), nil
	}
	return nil, errors.New("unknown speed unit")
}

// String turns a speed unit to string
func (sa IfSpeedPathUnit) String() string {
	got, _ := ifSpeedPathUnitMap.LoadValue(sa)
	return got
}

// UnmarshalText provides a speed unit from text
func (sa *IfSpeedPathUnit) UnmarshalText(input []byte) error {
	got, ok := ifSpeedPathUnitMap.LoadKey(string(input))
	if ok {
		*sa = got
		return nil
	}
	return errors.New("unknown provider")
}

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
				{"/state/port/ethernet/oper-speed", SpeedMegabits},
				{"/state/lag/bandwidth", SpeedBits},
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
				{"/interface/lag/lag-speed", SpeedBits},
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
				{"/interfaces/interface/aggregation/state/lag-speed", SpeedMegabits},
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
				{"/interfaces/interface/speed", SpeedBits},
			},
		},
	}
}

// ConfigurationUnmarshallerHook normalize gnmi configuration:
//   - replace an occurrence of "default" in the list of models with the list of default models.
func ConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeOf(Configuration{}) {
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
						newValue := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(new(interface{})).Elem()), 0, 0)
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
