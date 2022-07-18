// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"

	"akvorado/inlet/flow/input"
	"akvorado/inlet/flow/input/file"
	"akvorado/inlet/flow/input/udp"
)

// Configuration describes the configuration for the flow component
type Configuration struct {
	// Inputs define a list of input modules to enable
	Inputs []InputConfiguration
}

// DefaultConfiguration represents the default configuration for the flow component
func DefaultConfiguration() Configuration {
	return Configuration{
		Inputs: []InputConfiguration{{
			Decoder: "netflow",
			Config:  udp.DefaultConfiguration(),
		}, {
			Decoder: "sflow",
			Config:  udp.DefaultConfiguration(),
		}},
	}
}

// InputConfiguration represents the configuration for an input.
type InputConfiguration struct {
	// Decoder is the decoder to associate to the input.
	Decoder string
	// Config is the actual configuration of the input.
	Config input.Configuration
}

// ConfigurationUnmarshalerHook will help decode the Configuration
// structure by selecting the appropriate concrete type for
// input.Connfiguration, depending on the type contained in the
// source.
func ConfigurationUnmarshalerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if to.Type() != reflect.TypeOf(InputConfiguration{}) {
			return from.Interface(), nil
		}
		configField := to.FieldByName("Config")
		fromConfig := reflect.MakeMap(reflect.TypeOf(map[string]interface{}{}))

		// Find "type" key in map to get input type. Keep
		// "decoder" as is. Move everything else in "config".
		var inputType string
		if from.Kind() != reflect.Map {
			return nil, errors.New("input configuration should be a map")
		}
		mapKeys := from.MapKeys()
		for _, key := range mapKeys {
			var keyStr string
			// YAML may unmarshal keys to interfaces
			if key.Kind() == reflect.String {
				keyStr = key.String()
			} else if key.Kind() == reflect.Interface && key.Elem().Kind() == reflect.String {
				keyStr = key.Elem().String()
			} else {
				continue
			}
			switch strings.ToLower(keyStr) {
			case "type":
				inputTypeVal := from.MapIndex(key)
				if inputTypeVal.Kind() == reflect.Interface {
					inputTypeVal = inputTypeVal.Elem()
				}
				if inputTypeVal.Kind() != reflect.String {
					return nil, fmt.Errorf("type should be a string not %s", inputTypeVal.Kind())
				}
				inputType = strings.ToLower(inputTypeVal.String())
				from.SetMapIndex(key, reflect.Value{})
			case "decoder":
				// Leave as is
			case "config":
				return nil, errors.New("input configuration should not have a config key")
			default:
				fromConfig.SetMapIndex(reflect.ValueOf(keyStr), from.MapIndex(key))
				from.SetMapIndex(key, reflect.Value{})
			}
		}
		from.SetMapIndex(reflect.ValueOf("config"), fromConfig)

		if !configField.IsNil() && inputType == "" {
			// Get current type.
			currentType := configField.Elem().Type().Elem()
			for k, v := range inputs {
				if reflect.TypeOf(v()).Elem() == currentType {
					inputType = k
					break
				}
			}
		}
		if inputType == "" {
			return nil, errors.New("input configuration has no type")
		}

		// Get the appropriate input.Configuration for the string
		input, ok := inputs[inputType]
		if !ok {
			return nil, fmt.Errorf("%q is not a known input type", inputType)
		}

		// Alter config with a copy of the concrete type
		defaultV := input()
		original := reflect.Indirect(reflect.ValueOf(defaultV))
		if !configField.IsNil() && configField.Elem().Type().Elem() == reflect.TypeOf(defaultV).Elem() {
			// Use the value we already have instead of default.
			original = reflect.Indirect(configField.Elem())
		}
		copy := reflect.New(original.Type())
		copy.Elem().Set(reflect.ValueOf(original.Interface()))
		configField.Set(copy)

		// Resume decoding
		return from.Interface(), nil
	}
}

// MarshalYAML undoes ConfigurationUnmarshalerHook().
func (ic InputConfiguration) MarshalYAML() (interface{}, error) {
	var typeStr string
	for k, v := range inputs {
		if reflect.TypeOf(v()).Elem() == reflect.TypeOf(ic.Config).Elem() {
			typeStr = k
			break
		}
	}
	if typeStr == "" {
		return nil, errors.New("unable to guess input configuration type")
	}
	result := map[string]interface{}{
		"type":    typeStr,
		"decoder": ic.Decoder,
	}
	configStruct := reflect.ValueOf(ic.Config).Elem()
	for i, field := range reflect.VisibleFields(configStruct.Type()) {
		result[strings.ToLower(field.Name)] = configStruct.Field(i).Interface()
	}
	return result, nil
}

// MarshalJSON undoes ConfigurationUnmarshalerHook().
func (ic InputConfiguration) MarshalJSON() ([]byte, error) {
	result, err := ic.MarshalYAML()
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

var inputs = map[string](func() input.Configuration){
	"udp":  udp.DefaultConfiguration,
	"file": file.DefaultConfiguration,
}
