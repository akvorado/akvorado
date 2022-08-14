// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"strings"

	"github.com/mitchellh/mapstructure"
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
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.ComposeDecodeHookFunc(hooks...),
			mapstructure.ComposeDecodeHookFunc(mapstructureUnmarshallerHookFuncs...),
			mapstructure.TextUnmarshallerHookFunc(),
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
	}
}

// MapStructureMatchName tells if map key and field names are equal.
func MapStructureMatchName(mapKey, fieldName string) bool {
	key := strings.ToLower(strings.ReplaceAll(mapKey, "-", ""))
	field := strings.ToLower(fieldName)
	return key == field
}
