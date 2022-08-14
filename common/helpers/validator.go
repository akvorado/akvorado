// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"net"
	"reflect"
	"strconv"

	"github.com/go-playground/validator/v10"
)

// Validate is a validator instance to be used everywhere.
var Validate *validator.Validate

// RegisterSubnetMapValidation register a new SubnetMap[] type for
// validation. As validator requires an explicit type, we cannot just
// register all subnetmaps.
func RegisterSubnetMapValidation[V any]() {
	var zero SubnetMap[V]
	validatorFunc := func(field reflect.Value) interface{} {
		if subnetMap, ok := field.Interface().(SubnetMap[V]); ok {
			return subnetMap.ToMap()
		}
		return nil
	}
	Validate.RegisterCustomTypeFunc(validatorFunc, zero)
}

// isListen validates a <dns>:<port> combination for fields typically used for listening address
func isListen(fl validator.FieldLevel) bool {
	val := fl.Field().String()
	host, port, err := net.SplitHostPort(val)
	if err != nil {
		return false
	}
	// Port must be a iny <= 65535.
	if portNum, err := strconv.ParseInt(port, 10, 32); err != nil || portNum > 65535 || portNum < 0 {
		return false
	}

	// If host is specified, it should match a DNS name
	if host != "" {
		return Validate.Var(host, "hostname_rfc1123") == nil
	}
	return true
}

func init() {
	Validate = validator.New()
	Validate.RegisterValidation("listen", isListen)
	RegisterSubnetMapValidation[string]()
}
