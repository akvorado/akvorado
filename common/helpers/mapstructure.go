// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"strings"

	"github.com/mitchellh/mapstructure"
)

var mapstructureUnmarshallerHookFuncs = []mapstructure.DecodeHookFunc{}

// AddMapstructureUnmarshallerHook registers a new decoder hook for
// mapstructure. This should only be done during init.
func AddMapstructureUnmarshallerHook(hook mapstructure.DecodeHookFunc) {
	mapstructureUnmarshallerHookFuncs = append(mapstructureUnmarshallerHookFuncs, hook)
}

// GetMapStructureUnmarshallerHooks returns all the registered decode
// hooks for mapstructure.
func GetMapStructureUnmarshallerHooks() []mapstructure.DecodeHookFunc {
	return mapstructureUnmarshallerHookFuncs
}

// MapStructureMatchName tells if map key and field names are equal.
func MapStructureMatchName(mapKey, fieldName string) bool {
	key := strings.ToLower(strings.ReplaceAll(mapKey, "-", ""))
	field := strings.ToLower(fieldName)
	return key == field
}
