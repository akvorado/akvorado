// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package clickhouse

import (
	"fmt"
	"reflect"

	"akvorado/common/helpers"
)

func init() {
	helpers.AddPrettyFormatter(reflect.TypeOf(helpers.SubnetMap[NetworkAttributes]{}), fmt.Sprint)
}
