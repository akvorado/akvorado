// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-viper/mapstructure/v2"
)

var mapstructureUnmarshallerHookFuncs = []mapstructure.DecodeHookFunc{}

// RegisterMapstructureUnmarshallerHook registers a new decoder hook for
// mapstructure. This should only be done during init.
func RegisterMapstructureUnmarshallerHook(hook mapstructure.DecodeHookFunc) {
	mapstructureUnmarshallerHookFuncs = append(mapstructureUnmarshallerHookFuncs, hook)
}

// GetMapStructureDecoderConfig returns a decoder config for
// mapstructure with all registered hooks as well as appropriate
// default configuration.
func GetMapStructureDecoderConfig(config interface{}, hooks ...mapstructure.DecodeHookFunc) *mapstructure.DecoderConfig {
	return &mapstructure.DecoderConfig{
		Result:           config,
		ErrorUnused:      true,
		WeaklyTypedInput: true,
		MatchName:        MapStructureMatchName,
		DecodeHook: ProtectedDecodeHookFunc(
			mapstructure.ComposeDecodeHookFunc(
				mapstructure.ComposeDecodeHookFunc(hooks...),
				mapstructure.ComposeDecodeHookFunc(mapstructureUnmarshallerHookFuncs...),
				mapstructure.TextUnmarshallerHookFunc(),
				mapstructure.StringToTimeDurationHookFunc(),
				mapstructure.StringToSliceHookFunc(","),
			),
		),
	}
}

// ProtectedDecodeHookFunc wraps a DecodeHookFunc to recover and returns an error on panic.
func ProtectedDecodeHookFunc(hook mapstructure.DecodeHookFunc) mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (v interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				v = nil
				err = fmt.Errorf("internal error while parsing: %s", r)
			}
		}()
		return mapstructure.DecodeHookExec(hook, from, to)
	}
}

// MapStructureMatchName tells if map key and field names are equal.
func MapStructureMatchName(mapKey, fieldName string) bool {
	key := strings.ToLower(strings.ReplaceAll(mapKey, "-", ""))
	field := strings.ToLower(fieldName)
	return key == field
}

// DefaultValuesUnmarshallerHook adds default values from the provided
// configuration. For each missing non-default key, it will add them.
func DefaultValuesUnmarshallerHook[Configuration any](defaultConfiguration Configuration) mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		from = ElemOrIdentity(from)
		to = ElemOrIdentity(to)
		if to.Type() != reflect.TypeOf(defaultConfiguration) {
			return from.Interface(), nil
		}
		if from.Kind() != reflect.Map {
			return from.Interface(), nil
		}

		// Which field is not to the default value in the default configuration?
		found := map[string]bool{}
		defaultV := reflect.ValueOf(defaultConfiguration)
		for i := range defaultV.NumField() {
			if !defaultV.Field(i).IsZero() {
				found[defaultV.Type().Field(i).Name] = false
			}
		}
		mapKeys := from.MapKeys()
		for _, key := range mapKeys {
			var keyStr string
			if ElemOrIdentity(key).Kind() == reflect.String {
				keyStr = ElemOrIdentity(key).String()
			} else {
				continue
			}
			for fieldName := range found {
				if MapStructureMatchName(keyStr, fieldName) {
					found[fieldName] = true
				}
			}
		}
		for fieldName := range found {
			if !found[fieldName] {
				from.SetMapIndex(reflect.ValueOf(fieldName), defaultV.FieldByName(fieldName))
			}
		}
		return from.Interface(), nil
	}
}

// RenameKeyUnmarshallerHook move a configuration setting from one place to another.
func RenameKeyUnmarshallerHook[Configuration any](zeroConfiguration Configuration, fromLabel, toLabel string) mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeOf(zeroConfiguration) {
			return from.Interface(), nil
		}

		// country-database → geo-database
		var fromKey, toKey *reflect.Value
		fromMap := from.MapKeys()
		for i, k := range fromMap {
			k = ElemOrIdentity(k)
			if k.Kind() != reflect.String {
				return from.Interface(), nil
			}
			if MapStructureMatchName(k.String(), fromLabel) {
				fromKey = &fromMap[i]
			} else if MapStructureMatchName(k.String(), toLabel) {
				toKey = &fromMap[i]
			}
		}
		if fromKey != nil && toKey != nil {
			return nil, fmt.Errorf("cannot have both %q and %q", fromKey.String(), toKey.String())
		}
		if fromKey != nil {
			from.SetMapIndex(reflect.ValueOf(toLabel), from.MapIndex(*fromKey))
			from.SetMapIndex(*fromKey, reflect.Value{})
		}

		return from.Interface(), nil
	}
}

// ParametrizedConfigurationUnmarshallerHook will help decode a configuration
// structure parametrized by a type by selecting the appropriate concrete type
// depending on the type contained in the source. We have two configuration
// structures: the outer one should contain a "Config" field using the inner
// type. A map from configuration types to a function providing the inner
// default config should be provided.
func ParametrizedConfigurationUnmarshallerHook[OuterConfiguration any, InnerConfiguration any](zeroOuterConfiguration OuterConfiguration, innerConfigurationMap map[string](func() InnerConfiguration)) mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if to.Type() != reflect.TypeOf(zeroOuterConfiguration) {
			return from.Interface(), nil
		}
		configField := to.FieldByName("Config")
		fromConfig := reflect.MakeMap(reflect.TypeOf(gin.H{}))

		// Find "type" key in map to get input type. Keep existing fields as is.
		// Move everything else in "config".
		var innerConfigurationType string
		if from.Kind() != reflect.Map {
			return nil, errors.New("configuration should be a map")
		}
		mapKeys := from.MapKeys()
	outer:
		for _, key := range mapKeys {
			var keyStr string
			// YAML may unmarshal keys to interfaces
			if ElemOrIdentity(key).Kind() == reflect.String {
				keyStr = ElemOrIdentity(key).String()
			} else {
				continue
			}
			switch strings.ToLower(keyStr) {
			case "type":
				inputTypeVal := ElemOrIdentity(from.MapIndex(key))
				if inputTypeVal.Kind() != reflect.String {
					return nil, fmt.Errorf("type should be a string not %s", inputTypeVal.Kind())
				}
				innerConfigurationType = strings.ToLower(inputTypeVal.String())
				from.SetMapIndex(key, reflect.Value{})
			case "config":
				return nil, errors.New("configuration should not have a `config' key")
			default:
				t := to.Type()
				for i := range t.NumField() {
					if MapStructureMatchName(keyStr, t.Field(i).Name) {
						// Don't touch
						continue outer
					}
				}
				fromConfig.SetMapIndex(reflect.ValueOf(keyStr), from.MapIndex(key))
				from.SetMapIndex(key, reflect.Value{})
			}
		}
		from.SetMapIndex(reflect.ValueOf("config"), fromConfig)

		if !configField.IsNil() && innerConfigurationType == "" {
			// Get current type.
			currentType := configField.Elem().Type().Elem()
			for k, v := range innerConfigurationMap {
				typeOf := reflect.TypeOf(v())
				if typeOf.Kind() == reflect.Pointer {
					typeOf = typeOf.Elem()
				}
				if typeOf == currentType {
					innerConfigurationType = k
					break
				}
			}
		}
		if innerConfigurationType == "" {
			return nil, errors.New("configuration has no type")
		}

		// Get the appropriate input.Configuration for the string
		innerConfiguration, ok := innerConfigurationMap[innerConfigurationType]
		if !ok {
			return nil, fmt.Errorf("%q is not a known input type", innerConfigurationType)
		}

		// Alter config with a copy of the concrete type
		defaultV := innerConfiguration()
		original := reflect.Indirect(reflect.ValueOf(defaultV))
		if !configField.IsNil() && configField.Elem().Type() == reflect.TypeOf(defaultV) {
			// Use the value we already have instead of default.
			original = reflect.Indirect(configField.Elem())
		}
		copied := reflect.New(original.Type())
		copied.Elem().Set(reflect.ValueOf(original.Interface()))
		configField.Set(copied)

		// Resume decoding
		return from.Interface(), nil
	}
}

// ParametrizedConfigurationMarshalYAML undoes ParametrizedConfigurationUnmarshallerHook().
func ParametrizedConfigurationMarshalYAML[OuterConfiguration any, InnerConfiguration any](oc OuterConfiguration, innerConfigurationMap map[string](func() InnerConfiguration)) (interface{}, error) {
	var innerConfigStruct reflect.Value
	outerConfigStruct := ElemOrIdentity(reflect.ValueOf(oc))
	result := gin.H{}
	for i, field := range reflect.VisibleFields(outerConfigStruct.Type()) {
		if field.Name != "Config" {
			result[strings.ToLower(field.Name)] = outerConfigStruct.Field(i).Interface()
		} else {
			innerConfigStruct = outerConfigStruct.Field(i).Elem()
			if innerConfigStruct.Kind() == reflect.Pointer {
				innerConfigStruct = innerConfigStruct.Elem()
			}
		}
	}
	for k, v := range innerConfigurationMap {
		typeOf := reflect.TypeOf(v())
		if typeOf.Kind() == reflect.Pointer {
			typeOf = typeOf.Elem()
		}
		if typeOf == innerConfigStruct.Type() {
			result["type"] = k
			break
		}
	}
	if result["type"] == nil {
		return nil, errors.New("unable to guess configuration type")
	}
	for i, field := range reflect.VisibleFields(innerConfigStruct.Type()) {
		result[strings.ToLower(field.Name)] = innerConfigStruct.Field(i).Interface()
	}
	return result, nil
}
