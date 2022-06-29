// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"net"
	"strconv"

	"github.com/go-playground/validator/v10"
)

// Validate is a validator instance to be used everywhere.
var Validate *validator.Validate

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
}
