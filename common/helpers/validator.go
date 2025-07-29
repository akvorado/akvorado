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
	validatorFunc := func(field reflect.Value) any {
		switch subnetMap := field.Interface().(type) {
		case SubnetMap[V]:
			return subnetMap.ToMap()
		case *SubnetMap[V]:
			return subnetMap.ToMap()
		}
		return nil
	}
	Validate.RegisterCustomTypeFunc(validatorFunc, zero, &zero)
}

// netipValidation validates netip.Addr and netip.Prefix by turning them into a string.
func netipValidation(fl reflect.Value) any {
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
	// Port must be a <= 65535.
	if portNum, err := strconv.ParseInt(port, 10, 32); err != nil || portNum > 65535 || portNum < 0 {
		return false
	}

	// If host is specified, it should match a DNS name
	if host != "" {
		return Validate.Var(host, "hostname_rfc1123") == nil
	}
	return true
}

// noIntersectField validates a field value does not intersect with another one
// (both fields should be a slice)
func noIntersectField(fl validator.FieldLevel) bool {
	field := fl.Field()
	currentField, _, ok := fl.GetStructFieldOK()
	if !ok {
		return false
	}
	if field.Kind() != reflect.Slice || currentField.Kind() != reflect.Slice {
		return false
	}
	for i := range field.Len() {
		el1 := field.Index(i).Interface()
		for j := range currentField.Len() {
			el2 := currentField.Index(j).Interface()
			if el1 == el2 {
				return false
			}
		}
	}
	return true
}

func init() {
	Validate = validator.New()
	Validate.RegisterValidation("listen", isListen)
	Validate.RegisterValidation("ninterfield", noIntersectField)
	Validate.RegisterCustomTypeFunc(netipValidation, netip.Addr{}, netip.Prefix{})
	RegisterSubnetMapValidation[string]()
}
