// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package snmp

import (
	"fmt"
	"reflect"

	"akvorado/common/helpers"
)

func init() {
	helpers.AddPrettyFormatter(reflect.TypeOf(helpers.SubnetMap[Credentials]{}), fmt.Sprint)
}
