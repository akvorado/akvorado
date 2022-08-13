// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import "reflect"

// ElemOrIdentity returns Elem() of the provided value if this is an
// interface, else it returns the value unmodified.
func ElemOrIdentity(value reflect.Value) reflect.Value {
	if value.Kind() == reflect.Interface {
		return value.Elem()
	}
	return value
}
