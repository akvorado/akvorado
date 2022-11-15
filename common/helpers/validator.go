// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"net"
	"net/netip"
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
		switch subnetMap := field.Interface().(type) {
		case SubnetMap[V]:
			return subnetMap.ToMap()
		case *SubnetMap[V]:
			return subnetMap.ToMap()
		}
		return nil
	}
	Validate.RegisterCustomTypeFunc(validatorFunc, zero)
}

// netipValidation validates netip.Addr and netip.Prefix by turning them into a string.
func netipValidation(fl reflect.Value) interface{} {
	switch netipSomething := fl.Interface().(type) {
	case netip.Addr:
		if (netipSomething == netip.Addr{}) {
			return ""
		}
		return netipSomething.String()
	case netip.Prefix:
		if (netipSomething == netip.Prefix{}) {
			return ""
		}
		return netipSomething.String()
	}
	return nil
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
	Validate.RegisterCustomTypeFunc(netipValidation, netip.Addr{}, netip.Prefix{})
	RegisterSubnetMapValidation[string]()
}
